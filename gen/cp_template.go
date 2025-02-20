package gen

import (
	"bytes"
	"fmt"
	"github.com/Adtelligent/json-stream/reg"
	"reflect"
	"strings"
)

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
			template = fmt.Sprintf(`
				dst.%[1]s.CopyFrom(&src.%[1]s)
			`, field.Name)
		case reflect.Map:
			className := field.Type.String()
			if idx := strings.LastIndex(className, "."); idx != -1 {
				className = className[idx+1:]
			}
			template = fmt.Sprintf(mapCopyTemplate, field.Name, className)
		case reflect.Slice:
			if strings.HasPrefix(field.Type.String(), "[]*") {
				className := strings.Replace(field.Type.String(), "[]*", "", -1)
				if idx := strings.LastIndex(className, "."); idx != -1 {
					className = className[idx+1:]
				}
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

		result.WriteString(wrapCpTemplateWithCondition(structureName, template, field))
	}

	return result.Bytes(), nil
}

func wrapCpTemplateWithCondition(className string, template string, field reflect.StructField) string {
	fieldName := field.Name
	var condition string

	switch field.Type.Kind() {
	case reflect.Ptr:
		condition = fmt.Sprintf("src.%[1]s != nil && limiter.In(\"%[2]s.%[1]s\")", fieldName, className)
	case reflect.Struct:
		condition = fmt.Sprintf("&src.%[1]s != nil && limiter.In(\"%[2]s.%[1]s\")", fieldName, className)
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		condition = fmt.Sprintf("len(src.%[1]s) != 0 && limiter.In(\"%[2]s.%[1]s\")", fieldName, className)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		condition = fmt.Sprintf("src.%[1]s != 0 && limiter.In(\"%[2]s.%[1]s\")", fieldName, className)
	case reflect.Bool:
		condition = fmt.Sprintf("limiter.In(\"%[2]s.%[1]s\")", fieldName, className)
	default:
		return template
	}
	indentedTemplate := indentLines(template, "\t")
	return fmt.Sprintf("\tif %s {\n%s\t}\n", condition, indentedTemplate)
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

var sliceOfPointerCopyTemplate = `	dst.%[1]s = dst.%[1]s[:0]
	for _, d := range src.%[1]s {
		if d == nil {
			dst.%[1]s = append(dst.%[1]s, nil)
		} else {
			temp := d.Copy(limiter)
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
		dst.%[1]s = src.%[1]s.Copy(limiter)
	}
`

var copyFromTemplate = `
func (dst *{className}) Copy(limiter FieldsLimiter) *{className} {
	src := new({className})
{copyFunction}
	return src
}`

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
