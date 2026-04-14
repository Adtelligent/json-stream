package gen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
)

type FieldInfo struct {
	Name     string
	TypeStr  string // as written in source: "string", "*BidRequest", "[]int32", "map[string]*Foo"
	Tag      reflect.StructTag
	Kind     reflect.Kind // derived from TypeStr
	IsPtr    bool
	IsSlice  bool
	IsMap    bool
	MapKey   string // for map types: key type string e.g. "string", "int32"
	MapElem  string // for map types: element type string e.g. "*Foo", "string"
	ElemType string // for ptr/slice: element type name without leading * or []
}

// StructInfo replaces reflect.Type for struct inspection.
type StructInfo struct {
	Name   string
	Fields []FieldInfo
}

// Registry holds all parsed structs and their interface implementors.
type Registry struct {
	Structs map[string]*StructInfo
	// Implementors maps interface type name → list of struct names that implement it.
	// Detected by finding structs with a method named exactly like the interface type
	// (protobuf oneof: struct BidRequest_Site_ has method isBidRequest_DistributionchannelOneof()).
	Implementors map[string][]string
	// NamedTypes maps named type aliases (e.g. "ContextType") to their underlying reflect.Kind.
	// Used to resolve non-struct named types that appear as bare names in field declarations.
	NamedTypes map[string]reflect.Kind
}

// BuildRegistryFromAST parses Go source bytes and returns the populated registry.
func BuildRegistryFromAST(src []byte) (*Registry, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	reg := &Registry{
		Structs:      make(map[string]*StructInfo),
		Implementors: make(map[string][]string),
		NamedTypes:   make(map[string]reflect.Kind),
	}

	// Pass 1: collect all exported type definitions.
	// Sub-pass 1a: collect named non-struct types (e.g. "type ContextType int32").
	// Sub-pass 1b: collect struct definitions.
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
				continue // handled below
			}
			// Named non-struct type: record its underlying kind.
			underlyingStr := typeExprToString(typeSpec.Type)
			kind := deriveKindFromName(underlyingStr)
			if kind != reflect.Invalid {
				reg.NamedTypes[typeSpec.Name.Name] = kind
			}
		}
	}

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			si := &StructInfo{Name: typeSpec.Name.Name}
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue // embedded/anonymous field — skip
				}
				name := field.Names[0].Name
				if !ast.IsExported(name) {
					continue
				}
				typeStr := typeExprToString(field.Type)
				tag := ""
				if field.Tag != nil {
					tag = strings.Trim(field.Tag.Value, "`")
				}
				fi := buildFieldInfoWithRegistry(name, typeStr, tag, reg.NamedTypes)
				si.Fields = append(si.Fields, fi)
			}
			reg.Structs[si.Name] = si
		}
	}

	// Pass 2: collect method receivers to find interface implementors.
	// In protobuf oneof, a struct like BidRequest_Site_ has method isBidRequest_DistributionchannelOneof().
	// The method name equals the interface type name used in oneof fields.
	receiverMethods := make(map[string]map[string]bool) // structName -> set of method names
	for _, decl := range f.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		recv := funcDecl.Recv.List[0].Type
		var recvName string
		switch t := recv.(type) {
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				recvName = ident.Name
			}
		case *ast.Ident:
			recvName = t.Name
		}
		if recvName == "" {
			continue
		}
		if receiverMethods[recvName] == nil {
			receiverMethods[recvName] = make(map[string]bool)
		}
		receiverMethods[recvName][funcDecl.Name.Name] = true
	}

	// Pass 3: for each interface-typed field found in struct definitions,
	// collect all structs that have a method named after that interface type.
	interfaceFieldTypes := make(map[string]bool)
	for _, si := range reg.Structs {
		for _, fi := range si.Fields {
			if fi.Kind == reflect.Interface {
				interfaceFieldTypes[fi.TypeStr] = true
			}
		}
	}

	for ifaceType := range interfaceFieldTypes {
		var impls []string
		for structName, methods := range receiverMethods {
			if methods[ifaceType] {
				impls = append(impls, structName)
			}
		}
		if len(impls) > 0 {
			reg.Implementors[ifaceType] = impls
		}
	}

	return reg, nil
}

// typeExprToString converts an AST type expression to a canonical Go type string.
func typeExprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeExprToString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + typeExprToString(t.Elt)
		}
		return "[...]" + typeExprToString(t.Elt)
	case *ast.MapType:
		return "map[" + typeExprToString(t.Key) + "]" + typeExprToString(t.Value)
	case *ast.SelectorExpr:
		return typeExprToString(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		return "chan"
	case *ast.FuncType:
		return "func"
	default:
		return fmt.Sprintf("unknown(%T)", expr)
	}
}

func buildFieldInfoWithRegistry(name, typeStr, tag string, namedTypes map[string]reflect.Kind) FieldInfo {
	fi := FieldInfo{
		Name:    name,
		TypeStr: typeStr,
		Tag:     reflect.StructTag(tag),
	}
	fi.Kind, fi.IsPtr, fi.IsSlice, fi.IsMap, fi.ElemType, fi.MapKey, fi.MapElem = deriveKindWithRegistry(typeStr, namedTypes)
	return fi
}

func deriveKindWithRegistry(typeStr string, namedTypes map[string]reflect.Kind) (kind reflect.Kind, isPtr, isSlice, isMap bool, elemType, mapKey, mapElem string) {
	if strings.HasPrefix(typeStr, "[]") {
		isSlice = true
		inner := typeStr[2:]
		if strings.HasPrefix(inner, "*") {
			isPtr = true
			elemType = strings.TrimPrefix(inner, "*")
			if idx := strings.LastIndex(elemType, "."); idx >= 0 {
				elemType = elemType[idx+1:]
			}
		} else {
			elemType = inner
			if idx := strings.LastIndex(elemType, "."); idx >= 0 {
				elemType = elemType[idx+1:]
			}
		}
		kind = reflect.Slice
		return
	}

	if strings.HasPrefix(typeStr, "map[") {
		isMap = true
		kind = reflect.Map
		inner := typeStr[4:] // strip "map["
		depth := 1
		i := 0
		for i < len(inner) && depth > 0 {
			switch inner[i] {
			case '[':
				depth++
			case ']':
				depth--
			}
			i++
		}
		mapKey = inner[:i-1]
		mapElem = inner[i:]
		return
	}

	if strings.HasPrefix(typeStr, "*") {
		isPtr = true
		inner := strings.TrimPrefix(typeStr, "*")
		elemType = inner
		if idx := strings.LastIndex(elemType, "."); idx >= 0 {
			elemType = elemType[idx+1:]
		}
		kind = reflect.Ptr
		return
	}

	kind = deriveKindFromName(typeStr)
	if kind == reflect.Invalid && namedTypes != nil {
		lookup := typeStr
		if idx := strings.LastIndex(lookup, "."); idx >= 0 {
			lookup = lookup[idx+1:]
		}
		if k, ok := namedTypes[lookup]; ok {
			kind = k
			return
		}
	}
	if kind == reflect.Invalid {
		if strings.HasPrefix(typeStr, "is") {
			kind = reflect.Interface
		} else {
			kind = reflect.Struct
		}
	}
	return
}

// deriveKindFromName maps a Go primitive type name to its reflect.Kind.
// Returns reflect.Invalid for unknown/struct/interface types.
func deriveKindFromName(name string) reflect.Kind {
	// Strip package qualifier if present (e.g. "pkg.Type" → "Type").
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	switch name {
	case "string":
		return reflect.String
	case "bool":
		return reflect.Bool
	case "int":
		return reflect.Int
	case "int8":
		return reflect.Int8
	case "int16":
		return reflect.Int16
	case "int32":
		return reflect.Int32
	case "int64":
		return reflect.Int64
	case "uint":
		return reflect.Uint
	case "uint8":
		return reflect.Uint8
	case "uint16":
		return reflect.Uint16
	case "uint32":
		return reflect.Uint32
	case "uint64":
		return reflect.Uint64
	case "float32":
		return reflect.Float32
	case "float64":
		return reflect.Float64
	case "byte":
		return reflect.Uint8
	case "rune":
		return reflect.Int32
	case "interface{}":
		return reflect.Interface
	default:
		return reflect.Invalid
	}
}

// isStructType returns true if typeStr refers to a struct (not a primitive, slice, map, ptr, or interface).
// Used when we need to know if a bare type name (no prefix) is a struct.
func isStructType(typeStr string) bool {
	if strings.HasPrefix(typeStr, "[]") ||
		strings.HasPrefix(typeStr, "map[") ||
		strings.HasPrefix(typeStr, "*") {
		return false
	}
	k := deriveKindFromName(typeStr)
	if k != reflect.Invalid {
		return false
	}
	// Unexported "is*" names are interfaces.
	if strings.HasPrefix(typeStr, "is") {
		return false
	}
	return true
}

// isMapElemPtrToStruct returns true if a map element type string represents *SomeStruct.
func isMapElemPtrToStruct(mapElem string) bool {
	if !strings.HasPrefix(mapElem, "*") {
		return false
	}
	inner := strings.TrimPrefix(mapElem, "*")
	return isStructType(inner)
}

// isStructTypeWithRegistry returns true if typeStr refers to a struct,
// consulting namedTypes to exclude named primitive aliases like "type ContextType int32".
func isStructTypeWithRegistry(typeStr string, namedTypes map[string]reflect.Kind) bool {
	if strings.HasPrefix(typeStr, "[]") ||
		strings.HasPrefix(typeStr, "map[") ||
		strings.HasPrefix(typeStr, "*") {
		return false
	}
	k := deriveKindFromName(typeStr)
	if k != reflect.Invalid {
		return false
	}
	if namedTypes != nil {
		lookup := typeStr
		if idx := strings.LastIndex(lookup, "."); idx >= 0 {
			lookup = lookup[idx+1:]
		}
		if _, ok := namedTypes[lookup]; ok {
			return false // it's a named primitive alias, not a struct
		}
	}
	if strings.HasPrefix(typeStr, "is") {
		return false
	}
	return true
}
