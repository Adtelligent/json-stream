package gen

import (
	"bytes"
	"fmt"
	"github.com/Adtelligent/json-stream/reg"
	"reflect"
	"regexp"
	"strings"
)

var regexpForPackage = regexp.MustCompile(`\b\w+\.`)

func (f *SrcFile) getCopyFromImplementation(structureName string) ([]byte, error) {
	str := reg.TypeRegistry[structureName]

	var result bytes.Buffer
	var template string
	for i := 0; i < str.NumField(); i++ {
		field := str.Field(i)
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
			if valueType.Kind() == reflect.Ptr && valueType.Elem().Kind() == reflect.Struct {
				className = regexpForPackage.ReplaceAllString(className, "")
				template = fmt.Sprintf(mapCopyTemplateForPointer, field.Name, className)
			} else {
				template = fmt.Sprintf(mapCopyTemplate, field.Name, className)
			}
		case reflect.Slice:
			elemType := field.Type.Elem()
			if elemType.Kind() == reflect.Ptr && elemType.Elem().Kind() == reflect.Struct {
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

	return fmt.Sprintf("\tif !redefiner.Redefine(\"%[2]s.%[1]s\", path, unsafe.Pointer(&src.%[1]s), unsafe.Pointer(&dst.%[1]s)){\n%[3]s\t}\n", fieldName, className, indentedTemplate)
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
var mapCopyTemplateForPointer = `	if len(src.%[1]s) == 0 {
		if len(dst.%[1]s) != 0 {
			dst.%[1]s = make(%[2]s, len(dst.%[1]s))
		}
	} else {
		dst.%[1]s = make(%[2]s, len(src.%[1]s))
		for k, v := range src.%[1]s {
			if v == nil {
				dst.%[1]s[k] = nil
			} else {
				dst.%[1]s[k] = v.copy(redefiner, append(path, []byte(".%[1]s")...))
			}
		}
	}
`
var sliceOfPointerCopyTemplate = `	dst.%[1]s = dst.%[1]s[:0]
	for _, d := range src.%[1]s {
		if d == nil {
			dst.%[1]s = append(dst.%[1]s, nil)
		} else {
			temp := d.copy(redefiner, append(path, []byte(".%[1]s")...))
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
