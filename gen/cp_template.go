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
	for i := 0; i < str.NumField(); i++ {
		field := str.Field(i)
		if !field.IsExported() {
			continue
		}
		switch field.Type.Kind() {
		case reflect.Chan, reflect.Func, reflect.UnsafePointer, reflect.Interface:
			continue
		case reflect.Array:
			result.WriteString(fmt.Sprintf(`
				for i := range src.%[1]s {
					dst.%[1]s[i] = src.%[1]s[i]
				}
			`, field.Name))
		case reflect.Struct:
			result.WriteString(fmt.Sprintf(`
				dst.%[1]s.CopyFrom(&src.%[1]s)
			`, field.Name))
		case reflect.Map:
			className := field.Type.String()
			if field.Type.Elem().Kind() == reflect.Ptr && field.Type.Elem().Elem().Kind() == reflect.Struct {
				className = strings.Replace(className, f.PackageName+".", "", 1)
			}
			result.WriteString(fmt.Sprintf(mapCopyTemplate, field.Name, className))
		case reflect.Slice:
			if strings.HasPrefix(field.Type.String(), "[]*") {
				className := strings.Replace(field.Type.String(), "[]*", "", -1)
				if field.Type.Elem().Elem().Kind() == reflect.Struct {
					className = strings.Replace(className, f.PackageName+".", "", 1)
				}
				result.WriteString(fmt.Sprintf(sliceOfPointerCopyTemplate, field.Name, className))
			} else {
				result.WriteString(fmt.Sprintf("dst.%[1]s = append(dst.%[1]s[:0], src.%[1]s...)\n	", field.Name))
			}
		case reflect.Ptr:
			if field.Type.Elem().String() == "structpb.Struct" {
				result.WriteString(fmt.Sprintf(structpbCopyTemplate, field.Name))
			} else if field.Type.Elem().Kind() == reflect.Struct {
				fieldType := strings.Replace(field.Type.Elem().Name(), f.PackageName+".", "", 1)
				result.WriteString(fmt.Sprintf(pointerCopyTemplate, field.Name, fieldType))
			} else {
				result.WriteString(fmt.Sprintf("dst.%s = src.%s\n	", field.Name, field.Name))
			}
		default:
			result.WriteString(fmt.Sprintf("dst.%s = src.%s\n	", field.Name, field.Name))
		}
	}

	return result.Bytes(), nil
}

func generateCopyFromFile(className, copyFrom string) string {
	result := strings.ReplaceAll(copyFromTemplate, "{className}", className)
	result = strings.ReplaceAll(result, "{copyFrom}", copyFrom)
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

var mapCopyTemplate = `
	if len(src.%[1]s) == 0 {
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

var sliceOfPointerCopyTemplate = `
	dst.%[1]s = dst.%[1]s[:0]
	for _, d := range src.%[1]s {
		if d == nil {
			dst.%[1]s = append(dst.%[1]s, nil)
		} else {
			temp := new(%[2]s)
			temp.CopyFrom(d)
			dst.%[1]s = append(dst.%[1]s, temp)
		}
	}
`
var structpbCopyTemplate = `
	if src.%[1]s == nil {
		dst.%[1]s = nil
	} else {
		dst.%[1]s = proto.Clone(src.%[1]s).(*structpb.Struct)
	}
`
var pointerCopyTemplate = `
	if src.%[1]s == nil {
		dst.%[1]s = nil
	} else {
		if dst.%[1]s == nil {
			dst.%[1]s = new(%[2]s)
		}
		dst.%[1]s.CopyFrom(src.%[1]s)
	}
`

var copyFromTemplate = `
func (dst *{className}) CopyFrom(src *{className}) {
	{copyFrom}
}`

var marshalJsonTemplate = `
func (dst *{className}) MarshalJson() ([]byte, error) {
	var bb  bytes.Buffer
	write{qtcName}(&bb, dst, DefaultFieldsLimiter)
	return bb.Bytes(), nil
}

func (dst *{className}) WriteJsonTo(w io.Writer)  {
	write{qtcName}(w, dst, DefaultFieldsLimiter)
}

func (dst *{className}) MarshalJsonExtend(mask FieldsLimiter) ([]byte, error) {
	var bb  bytes.Buffer
	write{qtcName}(&bb, dst, mask)
	return bb.Bytes(), nil
}

func (dst *{className}) WriteJsonToExtend(w io.Writer, mask FieldsLimiter)  {
	write{qtcName}(w, dst, mask)
}
`
