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
	TypeStr  string
	Tag      reflect.StructTag
	Kind     reflect.Kind
	IsPtr    bool
	IsSlice  bool
	IsMap    bool
	MapKey   string
	MapElem  string
	ElemType string
}

type StructInfo struct {
	Name   string
	Fields []FieldInfo
}

type Registry struct {
	Structs      map[string]*StructInfo
	Implementors map[string][]string
	NamedTypes   map[string]reflect.Kind
}

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
				continue
			}
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
					continue
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

	receiverMethods := make(map[string]map[string]bool)
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
		inner := typeStr[4:]
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

func deriveKindFromName(name string) reflect.Kind {
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
	if strings.HasPrefix(typeStr, "is") {
		return false
	}
	return true
}

func isMapElemPtrToStruct(mapElem string) bool {
	if !strings.HasPrefix(mapElem, "*") {
		return false
	}
	inner := strings.TrimPrefix(mapElem, "*")
	return isStructType(inner)
}

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
			return false
		}
	}
	if strings.HasPrefix(typeStr, "is") {
		return false
	}
	return true
}
