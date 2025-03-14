package gen

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

var regexpForPackage = regexp.MustCompile(`\b\w+\.`)

func (f *SrcFile) getCopyFromImplementation(structureName string, strType reflect.Type) ([]byte, error) {
	var result bytes.Buffer
	var template string
	for i := 0; i < strType.NumField(); i++ {
		field := strType.Field(i)
		if !field.IsExported() {
			continue
		}
		switch field.Type.Kind() {
		case reflect.Chan, reflect.Func, reflect.UnsafePointer, reflect.Interface:
			continue
		case reflect.Array:
			template = fmt.Sprintf(`
				for i := range src.%[1]s {
					dst.%[1]s[i] = src.%[1]s[i]
				}
			`, field.Name)
		case reflect.Struct:
			template = fmt.Sprintf("\tdst.%[1]s = src.%[1]s.copy(redefiner, append(path, []byte(\".%[1]s\")...))\n", field.Name)
		case reflect.Map:
			valueType := field.Type.Elem()
			className := field.Type.String()
			if (valueType.Kind() == reflect.Ptr && valueType.Elem().Kind() == reflect.Struct) || valueType.Kind() == reflect.Struct {
				className = regexpForPackage.ReplaceAllString(className, "")
				switch field.Type.Key().Kind() {
				case reflect.String:
					template = fmt.Sprintf(mapStrCopyTemplateForPointer, field.Name, className)
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					template = fmt.Sprintf(mapIntCopyTemplateForPointer, field.Name, className)
				default:
					return nil, fmt.Errorf("unsupported map key type for field %s", field.Name)
				}
			} else {
				template = fmt.Sprintf(mapCopyTemplate, field.Name, className)
			}
		case reflect.Slice:
			elemType := field.Type.Elem()
			if (elemType.Kind() == reflect.Ptr && elemType.Elem().Kind() == reflect.Struct) || elemType.Kind() == reflect.Struct {
				className := elemType.Elem().Name()
				template = fmt.Sprintf(sliceOfPointerCopyTemplate, field.Name, className)
			} else {
				template = fmt.Sprintf("\tdst.%[1]s = append(dst.%[1]s[:0], src.%[1]s...)\n", field.Name)
			}
		case reflect.Ptr:
			if field.Type.Elem().String() == "structpb.Struct" {
				template = fmt.Sprintf(structpbCopyTemplate, field.Name)
			} else if field.Type.Elem().Kind() == reflect.Struct {
				fieldType := strings.Replace(field.Type.Elem().Name(), f.PackageName+".", "", 1)
				template = fmt.Sprintf(pointerCopyTemplate, field.Name, fieldType)
			} else {
				template = fmt.Sprintf("\tdst.%s = src.%s\n", field.Name, field.Name)
			}
		default:
			template = fmt.Sprintf("\tdst.%s = src.%s\n", field.Name, field.Name)
		}

		result.WriteString(wrapCpTemplateWithRedefiner(structureName, template, field))
	}
	return result.Bytes(), nil
}

func wrapCpTemplateWithRedefiner(className string, template string, field reflect.StructField) string {
	fieldName := field.Name
	indentedTemplate := indentLines(template, "\t")

	return fmt.Sprintf("\tif !redefiner.Redefine(\"%[2]s.%[1]s\", path, []byte(\"%[1]s\"), unsafe.Pointer(&src.%[1]s), unsafe.Pointer(&dst.%[1]s)){\n%[3]s\t}\n", fieldName, className, indentedTemplate)
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

func generateCopyFunction(className, copyFunction string) string {
	result := strings.ReplaceAll(copyFromTemplate, "{className}", className)
	result = strings.ReplaceAll(result, "{copyFunction}", copyFunction)
	return result
}

func generateMarshalJsonFile(className string) string {
	qtcName := generateQtcName(className)
	result := strings.ReplaceAll(marshalJsonTemplate, "{className}", className)
	result = strings.ReplaceAll(result, "{qtcName}", qtcName)
	return result
}

func generateQtcName(className string) string {
	return strings.ToLower(className[:1]) + className[1:] + "JSON"
}

func generateGetValueUnsafePointerMethod(className string, strType reflect.Type) string {
	var caseEntries []string
	for i := 0; i < strType.NumField(); i++ {
		field := strType.Field(i)
		if !field.IsExported() {
			continue
		}
		var caseCode string
		fieldCaseHeader := fmt.Sprintf("\tif bytes.Equal(parts[1],[]byte(\"%s\")) {\n", field.Name)
		switch field.Type.Kind() {
		case reflect.Chan, reflect.Func, reflect.UnsafePointer, reflect.Interface:
			continue
		case reflect.Struct:
			caseCode = fmt.Sprintf(structGetValueTemplate, field.Name)
		case reflect.Ptr:
			if field.Type.Elem().Kind() == reflect.Struct && field.Type.Elem().String() != "structpb.Struct" {
				caseCode = fmt.Sprintf(ptrStructGetValueTemplate, field.Name)
			} else {
				caseCode = fmt.Sprintf(ptrGetValueTemplate, field.Name)
			}
		case reflect.Slice:
			if field.Type.Elem().Kind() == reflect.Ptr && field.Type.Elem().Elem().Kind() == reflect.Struct {
				caseCode = fmt.Sprintf(sliceStructGetValueTemplate, field.Name)
			} else {
				caseCode = fmt.Sprintf(sliceGetValueTemplate, field.Name)
			}
		case reflect.Map:
			switch field.Type.Key().Kind() {
			case reflect.String:
				if field.Type.Elem().Kind() == reflect.Ptr && field.Type.Elem().Elem().Kind() == reflect.Struct {
					caseCode = fmt.Sprintf(mapStrStructGetValueTemplate, field.Name)
				} else {
					caseCode = fmt.Sprintf(mapStrGetValueTemplate, field.Name)
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				keyTypeName := field.Type.Key().String()
				if field.Type.Elem().Kind() == reflect.Ptr && field.Type.Elem().Elem().Kind() == reflect.Struct {
					caseCode = fmt.Sprintf(mapIntStructGetValueTemplate, field.Name, keyTypeName)
				} else {
					caseCode = fmt.Sprintf(mapIntGetValueTemplate, field.Name, keyTypeName)
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				keyTypeName := field.Type.Key().String()
				if field.Type.Elem().Kind() == reflect.Ptr && field.Type.Elem().Elem().Kind() == reflect.Struct {
					caseCode = fmt.Sprintf(mapUintStructGetValueTemplate, field.Name, keyTypeName)
				} else {
					caseCode = fmt.Sprintf(mapUintGetValueTemplate, field.Name, keyTypeName)
				}
			default:
				caseCode = fmt.Sprintf(`	return nil, nil, fmt.Errorf("unsupported map key type for field %[1]s", "%[1]s")
`, field.Name)
			}
		default:
			caseCode = fmt.Sprintf(defaultGetValueTemplate, field.Name)
		}
		caseEntry := fieldCaseHeader + caseCode + "\n\t}\n"
		caseEntries = append(caseEntries, caseEntry)
	}
	mapCases := strings.Join(caseEntries, "\n")
	methodCode := strings.ReplaceAll(getUnsafePointerTemplate, "{cases}", mapCases)
	methodCode = strings.ReplaceAll(methodCode, "{className}", className)
	return methodCode
}

var mapCopyTemplate = `	if len(src.%[1]s) == 0 {
		if len(dst.%[1]s) != 0 {
			dst.%[1]s = make(%[2]s, len(dst.%[1]s))
		}
	} else {
		dst.%[1]s = make(%[2]s, len(src.%[1]s))

		for k, v := range src.%[1]s {
			dst.%[1]s[k] = v
		}
	}
`
var mapStrCopyTemplateForPointer = `	if len(src.%[1]s) == 0 {
		if len(dst.%[1]s) != 0 {
			dst.%[1]s = make(%[2]s, len(dst.%[1]s))
		}
	} else {
		dst.%[1]s = make(%[2]s, len(src.%[1]s))
		for k, v := range src.%[1]s {
			if v == nil {
				dst.%[1]s[k] = nil
			} else {
				dst.%[1]s[k] = v.copy(redefiner, append(append(path, []byte(".%[1]s")...), []byte(k)...))
			}
		}
	}
`

var mapIntCopyTemplateForPointer = `	if len(src.%[1]s) == 0 {
		if len(dst.%[1]s) != 0 {
			dst.%[1]s = make(%[2]s, len(dst.%[1]s))
		}
	} else {
		dst.%[1]s = make(%[2]s, len(src.%[1]s))
		for k, v := range src.%[1]s {
			if v == nil {
				dst.%[1]s[k] = nil
			} else {
				dst.%[1]s[k] = v.copy(redefiner, append(append(path, []byte(".%[1]s.")...), strconv.Itoa(i)...))
			}
		}
	}
`

var sliceOfPointerCopyTemplate = `	dst.%[1]s = dst.%[1]s[:0]
	for i, d := range src.%[1]s {
		if d == nil {
			dst.%[1]s = append(dst.%[1]s, nil)
		} else {
			temp := d.copy(redefiner, append(append(path, []byte(".%[1]s.")...), strconv.Itoa(i)...))
			dst.%[1]s = append(dst.%[1]s, temp)
		}
	}
`
var structpbCopyTemplate = `	if src.%[1]s == nil {
		dst.%[1]s = nil
	} else {
		dst.%[1]s = proto.Clone(src.%[1]s).(*structpb.Struct)
	}
`
var pointerCopyTemplate = `	if src.%[1]s == nil {
		dst.%[1]s = nil
	} else {
		dst.%[1]s = src.%[1]s.copy(redefiner, append(path, []byte(".%[1]s")...))
	}
`
var copyFromTemplate = `
func (src *{className}) copy(redefiner FieldRedefiner, path []byte) *{className} {
    dst := new({className})
{copyFunction}
    return dst
}

func (src *{className}) Copy(redefiner FieldRedefiner) *{className} {
	initPath := getSliceByte()
	defer func() {
		putSliceByte(initPath)
	}()
	initPath = append(initPath, []byte("{className}")...)
    return src.copy(redefiner, initPath)
}
`

var getUnsafePointerTemplate = `
func (obj *{className}) GetValueUnsafePointer(pathToField []byte) (unsafe.Pointer, reflect.Type, error) {
	parts := bytes.Split(pathToField, []byte("."))
	if len(parts) == 0 {
		return nil, nil, fmt.Errorf("empty path")
	}
	if !bytes.Equal(parts[0], []byte("{className}")) {
		return nil, nil, fmt.Errorf("incorrect path: %s", parts[0])
	}
	return obj.getValueUnsafePointer(parts)
}

func (obj *{className}) getValueUnsafePointer(parts [][]byte) (unsafe.Pointer, reflect.Type, error) {
{cases}
	return nil, nil, fmt.Errorf("field not found: %s", parts[1])

}
`
var structGetValueTemplate = `			if len(parts) > 2 {
				return obj.%[1]s.getValueUnsafePointer(parts[1:])
			}
			return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil`
var ptrStructGetValueTemplate = `		if obj.%[1]s == nil {
			return nil, nil, fmt.Errorf("field %[1]s is nil")
		}
		if len(parts) > 2 {
			return obj.%[1]s.getValueUnsafePointer(parts[1:])
		}
		return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil`
var ptrGetValueTemplate = `		if len(parts) > 2 {
			return nil, nil, fmt.Errorf("field %%s is not a nested structure", parts[0])
		}
		if obj.%[1]s == nil {
			return nil, nil, fmt.Errorf("field %[1]s is nil")
		}
		return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil`
var sliceStructGetValueTemplate = `		if len(parts) < 3 {
			return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil
		}
		index, err := strconv.Atoi(unsafe.String(unsafe.SliceData(parts[2]), len(parts[2])))
		if err != nil || index < 0 || index >= len(obj.%[1]s) {
			return nil, nil, fmt.Errorf("invalid index for slice %[1]s: %%d", index)
		}
		if obj.%[1]s[index] == nil {
			return nil, nil, fmt.Errorf("nil element at index %%d in slice %[1]s", index)
		}
		if len(parts) > 3 {
			return obj.%[1]s[index].getValueUnsafePointer(parts[2:])
		}
		return unsafe.Pointer(&obj.%[1]s[index]), GetType("{className}.%[1]s"), nil`
var sliceGetValueTemplate = `		if len(parts) < 3 {
			return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil
		}
		index, err := strconv.Atoi(unsafe.String(unsafe.SliceData(parts[1]), len(parts[1])))
		if err != nil || index < 0 || index >= len(obj.%[1]s) {
			return nil, nil, fmt.Errorf("invalid index for slice %[1]s: %%d", index)
		}
		if len(parts) > 3 {
			return nil, nil, fmt.Errorf("field %%s is not a nested structure", parts[0])
		}
		return unsafe.Pointer(&obj.%[1]s[index]), GetType("{className}.%[1]s"), nil`
var mapStrStructGetValueTemplate = `		if len(parts) < 3 {
			return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil
		}
		key := unsafe.String(unsafe.SliceData(parts[2]), len(parts[2]))
		value, ok := obj.%[1]s[key]
		if !ok {
			return nil, nil, fmt.Errorf("key not found in map %[1]s: %%s", key)
		}
		if value == nil {
			return nil, nil, fmt.Errorf("nil value for key %%s in map %[1]s", key)
		}
		if len(parts) > 3 {
			return value.getValueUnsafePointer(parts[2:])
		}
		return unsafe.Pointer(&value), GetType("{className}.%[1]s"), nil`
var mapStrGetValueTemplate = `		if len(parts) < 3 {
			return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil
		}
		key := unsafe.String(unsafe.SliceData(parts[2]), len(parts[2]))
		value, ok := obj.%[1]s[key]
		if !ok {
			return nil, nil, fmt.Errorf("key not found in map %[1]s: %%s", key)
		}
		if len(parts) > 3 {
			return nil, nil, fmt.Errorf("field %%s is not a nested structure", parts[0])
		}
		return unsafe.Pointer(&value), GetType("{className}.%[1]s"), nil`

var mapIntStructGetValueTemplate = `		if len(parts) < 3 {
			return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil
		}
		keyInt, err := strconv.Atoi(unsafe.String(unsafe.SliceData(parts[2]), len(parts[2])))
		if err != nil {
			return nil, nil, fmt.Errorf("invalid key for map %[1]s: %%s", parts[2])
		}
		key := %[2]s(keyInt)
		value, ok := obj.%[1]s[key]
		if !ok {
			return nil, nil, fmt.Errorf("key not found in map %[1]s: %%d", key)
		}
		if value == nil {
			return nil, nil, fmt.Errorf("nil value for key %%v in map %[1]s", key)
		}
		if len(parts) > 3 {
			return value.getValueUnsafePointer(parts[2:])
		}
		return unsafe.Pointer(&value), GetType("{className}.%[1]s"), nil`
var defaultGetValueTemplate = `		if len(parts) > 3 {
			return nil, nil, fmt.Errorf("field %%s is not a nested structure", parts[1])
		}
		return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil`
var mapUintGetValueTemplate = `		if len(parts) < 3 {
			return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil
		}
		keyUint, err := strconv.ParseUint(parts[2], 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid key for map %[1]s: %%s", parts[1])
		}
		key := %[2]s(keyUint)
		value, ok := obj.%[1]s[key]
		if !ok {
			return nil, nil, fmt.Errorf("key not found in map %[1]s: %%d", key)
		}
		if len(parts) > 3 {
			return nil, nil, fmt.Errorf("field %%s is not a nested structure", parts[0])
		}
		return unsafe.Pointer(&value), GetType("{className}.%[1]s"), nil`
var mapUintStructGetValueTemplate = `		if len(parts) < 3 {
			return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil
		}
		keyUint, err := strconv.ParseUint(parts[2], 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid key for map %[1]s: %%s", parts[1])
		}
		key := %[2]s(keyUint)
		value, ok := obj.%[1]s[key]
		if !ok {
			return nil, nil, fmt.Errorf("key not found in map %[1]s: %%d", key)
		}
		if value == nil {
			return nil, nil, fmt.Errorf("nil value for key %%v in map %[1]s", key)
		}
		if len(parts) > 3 {
			return value.getValueUnsafePointer(parts[2:])
		}
		return unsafe.Pointer(&value), GetType("{className}.%[1]s"), nil`
var mapIntGetValueTemplate = `		if len(parts) < 3 {
			return unsafe.Pointer(&obj.%[1]s), GetType("{className}.%[1]s"), nil
		}
		keyInt, err := strconv.Atoi(unsafe.String(unsafe.SliceData(parts[2]), len(parts[2])))
		if err != nil {
			return nil, nil, fmt.Errorf("invalid key for map %[1]s: %%s", parts[2])
		}
		key := %[2]s(keyInt)
		value, ok := obj.%[1]s[key]
		if !ok {
			return nil, nil, fmt.Errorf("key not found in map %[1]s: %%d", key)
		}
		if len(parts) > 3 {
			return nil, nil, fmt.Errorf("field %%s is not a nested structure", parts[1])
		}
		return unsafe.Pointer(&value), GetType("{className}.%[1]s"), nil`

var marshalJsonTemplate = `
func (dst *{className}) MarshalJson() ([]byte, error) {
	var bb  bytes.Buffer
	write{qtcName}(&bb, dst)
	return bb.Bytes(), nil
}

func (dst *{className}) WriteJsonTo(w io.Writer)  {
	write{qtcName}(w, dst)
}
`
