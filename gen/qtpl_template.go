package gen

import (
	"bytes"
	"flag"
	"fmt"
	"reflect"
	"strings"

	"github.com/Adtelligent/json-stream/reg"
)

var boolToInt = flag.Bool("boolToInt", false, "bool to int generation")

func GetQTPLFile(className string, f *SrcFile) (string, error) {
	res, err := getWriteJSON(className, f)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(qtcFileContentTemplate, generateQtcName(className), className, res), nil
}

func parseJSONNameFromProtoTag(tag string) string {
	nameIndex := strings.Index(tag, "json=")
	if nameIndex == -1 {
		return ""
	}

	nameIndex += len("json=")

	comma := strings.IndexRune(tag[nameIndex:], ',')
	if comma == -1 {
		return ""
	}

	return tag[nameIndex : nameIndex+comma]
}

func parseNameFromProtoTag(tag string) string {
	nameIndex := strings.Index(tag, "name=")
	if nameIndex == -1 {
		return ""
	}

	nameIndex += len("name=")

	comma := strings.IndexRune(tag[nameIndex:], ',')
	if comma == -1 {
		return ""
	}

	return tag[nameIndex : nameIndex+comma]
}

func getJsonName(field reflect.StructField) string {
	jsonName := parseJSONNameFromProtoTag(field.Tag.Get("protobuf"))
	if jsonName != "" {
		return jsonName
	}

	jsonName = parseNameFromProtoTag(field.Tag.Get("protobuf"))
	if jsonName != "" {
		return jsonName
	}

	return strings.Replace(field.Tag.Get("json"), ",omitempty", "", -1)
}

func getWriteJSON(className string, f *SrcFile) (string, error) {
	if f.isImplementator(className) {
		return getImplementatorJSON(className, f)
	} else {
		return getStructureJSON(className, f)
	}
}

func wrapTemplateWithCondition(template string, field reflect.StructField, className string) string {
	fieldName := field.Name
	var condition string

	switch field.Type.Kind() {
	case reflect.Ptr, reflect.Interface:
		condition = fmt.Sprintf("d.%[2]s != nil && mask.In(\"%[1]s.%[2]s\")", className, fieldName)
	case reflect.Struct:
		condition = fmt.Sprintf("&d.%[2]s != nil && mask.In(\"%[1]s.%[2]s\")", className, fieldName)
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		condition = fmt.Sprintf("len(d.%[2]s) != 0 && mask.In(\"%[1]s.%[2]s\")", className, fieldName)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		condition = fmt.Sprintf("d.%[2]s != 0 && mask.In(\"%[1]s.%[2]s\")", className, fieldName)
	default:
		return template
	}
	return fmt.Sprintf("{%% if %s %%}%s{%% endif %%}\n", condition, template)
}

func wrapTemplateWithConditionForImplementator(template string, field reflect.StructField, className string) string {
	fieldName := field.Name
	condition := fmt.Sprintf("mask.In(\"%[1]s.%[2]s\")", className, fieldName)
	return fmt.Sprintf("{%% if %s %%}%s{%% endif %%}\n", condition, template)
}

func replaceMacros(template string, className string, field reflect.StructField) string {
	template = wrapTemplateWithCondition(template, field, className)
	template = strings.Replace(template, "{fieldName}", field.Name, -1)
	template = strings.Replace(template, "{className}", className, -1)
	template = strings.Replace(template, "{qtplFunc}", generateQtcName(getPrintableClassName(field.Type.String())), -1)
	return template
}

func replaceMacrosForImplementator(template string, className string, field reflect.StructField) string {
	template = wrapTemplateWithConditionForImplementator(template, field, className)
	template = strings.Replace(template, "{fieldName}", field.Name, -1)
	template = strings.Replace(template, "{className}", className, -1)
	template = strings.Replace(template, "{qtplFunc}", generateQtcName(getPrintableClassName(field.Type.String())), -1)
	return template
}

func getPrintableClassName(t string) string {
	dotIndex := strings.IndexRune(t, '.')
	if dotIndex == -1 {
		return t
	}

	return t[dotIndex+1:]
}

func getStructureJSON(className string, f *SrcFile) (string, error) {
	str := reg.TypeRegistry[className]
	var result bytes.Buffer
	result.WriteString("{\n")
	result.WriteString("		{% code comma := false %}\n")

	for i := 0; i < str.NumField(); i++ {
		field := str.Field(i)
		jsonName := getJsonName(field)

		if jsonName == "" && field.Type.Kind() != reflect.Interface {
			continue
		}

		fieldTemplate, err := generateFieldTemplate(field.Type, field, f, jsonName)
		if err != nil {
			return "", fmt.Errorf("error generating template for field %s: %w", field.Name, err)
		}

		result.WriteString(replaceMacros(fieldTemplate, className, field))
	}

	result.WriteString("	}")

	return result.String(), nil
}

func getImplementatorJSON(className string, f *SrcFile) (string, error) {
	var result bytes.Buffer
	str := reg.TypeRegistry[className]
	for i := 0; i < str.NumField(); i++ {
		field := str.Field(i)
		jsonName := getJsonName(field)

		if jsonName == "" && field.Type.Kind() != reflect.Interface {
			continue
		}

		fieldTemplate, err := generateFieldTemplateForImplementator(field.Type, field, f, jsonName)
		if err != nil {
			return "", fmt.Errorf("error generating template for field %s: %w", field.Name, err)
		}

		result.WriteString(replaceMacrosForImplementator(fieldTemplate, className, field))
	}
	return result.String(), nil
}

func generateFieldTemplate(typ reflect.Type, field reflect.StructField, f *SrcFile, jsonName string) (string, error) {
	fieldName := formatFieldName(typ, field.Name)
	template, err := generateInnerFieldTemplate(typ, fieldName, f)
	if err != nil {
		return "", err
	}
	wrappedTemplate := formatTemplate(jsonName, template)
	return wrappedTemplate, nil
}

func generateFieldTemplateForImplementator(typ reflect.Type, field reflect.StructField, f *SrcFile, jsonName string) (string, error) {
	fieldName := formatFieldName(typ, field.Name)
	template, err := generateInnerFieldTemplate(typ, fieldName, f)
	if err != nil {
		return "", err
	}

	if typ.Kind() == reflect.Ptr && jsonName != "" {
		fullTemplate := fmt.Sprintf("{%% if d.%s != nil %%}\"%s\":%s{%% endif %%}", field.Name, jsonName, template)
		return fullTemplate, nil
	}

	wrappedTemplate := formatTemplateForImplementator(jsonName, template)
	return wrappedTemplate, nil
}

func formatFieldName(typ reflect.Type, fieldName string) string {
	if typ.Kind() == reflect.Ptr {
		return "*d." + fieldName
	} else if typ.Kind() == reflect.Struct {
		return "&d." + fieldName
	}
	return "d." + fieldName
}

func formatTemplate(jsonName, template string) string {
	if jsonName == "" {
		return strings.ReplaceAll(`
			{% if comma %} , {% endif %}
			{innerTemplate}
			{% code comma = true %}
		`,
			"{innerTemplate}", template)
	}
	return strings.ReplaceAll(strings.ReplaceAll(`
			{% if comma %} , {% endif %}
			"{jsonFieldName}":{innerTemplate}
			{% code comma = true %}
		`,
		"{innerTemplate}", template),
		"{jsonFieldName}", jsonName)
}

func formatTemplateForImplementator(jsonName, template string) string {
	if jsonName == "" {
		return template
	}
	return strings.ReplaceAll(
		strings.ReplaceAll(`"{jsonFieldName}":{innerTemplate}`, "{innerTemplate}", template),
		"{jsonFieldName}", jsonName,
	)
}

func generateInnerFieldTemplate(typ reflect.Type, fieldName string, f *SrcFile) (string, error) {
	if typ.Kind() == reflect.Interface {
		var bb bytes.Buffer
		implementators := f.findInterfaceImplementators(typ)
		bb.WriteString("{% ")
		for i, impl := range implementators {
			bb.WriteString(fmt.Sprintf("if v%d, ok := %s.(*%s); ok %%}\n", i, fieldName, impl))
			bb.WriteString(fmt.Sprintf("				{%%= %s(v%d, mask) %%}\n", generateQtcName(impl), i))
			bb.WriteString("			{% else")
		}

		bb.WriteString(" %}\n")
		bb.WriteString(fmt.Sprintf("				{%%code log.Fatalf(\"unknown interface implementator for field %[1]s. Value: %%+v\", %[1]s) %%}\n", fieldName))
		bb.WriteString("			{% endif %}")

		return bb.String(), nil
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return replaceTemplate(intQTPLFormatInnerTemplate, fieldName), nil
	case reflect.String:
		return replaceTemplate(stringQTPLFormatInnerTemplate, fieldName), nil
	case reflect.Bool:
		if *boolToInt {
			return replaceTemplate(boolToIntQTPLFormatInnerTemplate, fieldName), nil
		}
		return replaceTemplate(boolQTPLFormatInnerTemplate, fieldName), nil
	case reflect.Float32, reflect.Float64:
		return replaceTemplate(floatQTPLFormatInnerTemplate, fieldName), nil
	case reflect.Slice, reflect.Array:
		return generateSliceTemplate(typ, fieldName, f)
	case reflect.Struct:
		return generateStructTemplate(typ, fieldName)
	case reflect.Ptr:
		return generatePointerTemplate(typ, fieldName, f)
	case reflect.Map:
		return generateMapTemplate(typ, fieldName, f)
	default:
		return "", fmt.Errorf("unsupported type: %v", typ.Kind())
	}
}

func replaceTemplate(template, fieldName string) string {
	return strings.ReplaceAll(template, "{fieldName}", fieldName)
}

func generateSliceTemplate(typ reflect.Type, fieldName string, f *SrcFile) (string, error) {
	elemType := typ.Elem()
	var err error
	var nestedTemplate string
	if elemType.Kind() == reflect.Struct {
		nestedTemplate, err = generateInnerFieldTemplate(elemType, "&v", f)
	} else {
		nestedTemplate, err = generateInnerFieldTemplate(elemType, "v", f)
	}

	if err != nil {
		return "", err
	}
	totalVar := "total" + strings.ReplaceAll(fieldName, ".", "")
	template := replaceTemplate(sliceQTPLFormatInnerTemplate, fieldName)
	template = strings.ReplaceAll(template, "{totalVar}", totalVar)
	template = strings.ReplaceAll(template, "{nestedTemplate}", nestedTemplate)
	return template, nil
}

func generateStructTemplate(typ reflect.Type, fieldName string) (string, error) {
	fieldName = strings.ReplaceAll(fieldName, "*", "")
	if _, ok := reg.TypeRegistry[typ.Name()]; !ok {
		if typ.String() == "structpb.Struct" {
			return replaceTemplate(structpbQTPLFormatTemplate, fieldName), nil
		}
		return "", fmt.Errorf("unknown struct %s", typ.Name())
	}
	return replaceTemplate(structQTPLFormatInnerTemplate, fieldName), nil
}

func generatePointerTemplate(typ reflect.Type, fieldName string, f *SrcFile) (string, error) {
	elemType := typ.Elem()
	nestedTemplate, err := generateInnerFieldTemplate(elemType, fieldName, f)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(pointerQTPLFormatInnerTemplate, "{nestedTemplate}", nestedTemplate), nil
}

func generateMapTemplate(typ reflect.Type, fieldName string, f *SrcFile) (string, error) {

	var err error
	var keyTemplate string
	if typ.Key().Kind() == reflect.Struct {
		keyTemplate, err = generateInnerFieldTemplate(typ.Key(), "&k", f)
	} else {
		keyTemplate, err = generateInnerFieldTemplate(typ.Key(), "k", f)
	}
	if err != nil {
		return "", err
	}

	if typ.Key().Kind() != reflect.String {
		keyTemplate = `"` + keyTemplate + `"`
	}

	var valueTemplate string
	if typ.Elem().Kind() == reflect.Struct {
		valueTemplate, err = generateInnerFieldTemplate(typ.Elem(), "&v", f)
	} else {
		valueTemplate, err = generateInnerFieldTemplate(typ.Elem(), "v", f)
	}
	if err != nil {
		return "", err
	}

	totalVar := "total" + strings.ReplaceAll(fieldName, ".", "")
	template := replaceTemplate(mapQTPLFormatInnerTemplate, fieldName)
	template = strings.ReplaceAll(template, "{keyTemplate}", keyTemplate)
	template = strings.ReplaceAll(template, "{totalVar}", totalVar)
	template = strings.ReplaceAll(template, "{valueTemplate}", valueTemplate)
	return template, nil
}

var intQTPLFormatInnerTemplate = `{%d= int({fieldName}) %}`

var stringQTPLFormatInnerTemplate = `{%q= {fieldName} %}`

var boolQTPLFormatInnerTemplate = `{% if {fieldName} %} true {% else %} false {% endif %}`
var boolToIntQTPLFormatInnerTemplate = `{% if {fieldName} %} 1 {% else %} 0 {% endif %}`

var floatQTPLFormatInnerTemplate = `{%f= float64({fieldName}) %}`

var sliceQTPLFormatInnerTemplate = `{% code {totalVar} := len({fieldName}) %}
			[
				{% for i, v := range {fieldName} %}
					{nestedTemplate}
					{% if i + 1 < {totalVar} %} , {% endif %}
				{% endfor %}
			]`

var structQTPLFormatInnerTemplate = `{%= {qtplFunc}({fieldName}, mask) %}`

var structpbQTPLFormatTemplate = `
			{% code
				extB, err := {fieldName}.MarshalJSON()
				if err != nil {
					log.Printf("cant marshal {fieldName} %v. Err: %s\n", {fieldName}, err)
				}
			%}
			{%z= extB %}
			{% code comma = true %}
`

var pointerQTPLFormatInnerTemplate = `{nestedTemplate}`

var mapQTPLFormatInnerTemplate = `{
				{% code
                    {totalVar} := len({fieldName})
                    i := 0
                %}
				{% for k, v := range {fieldName} %}
					{keyTemplate}:{valueTemplate}
					{% code i++ %}
					{% if i < {totalVar} %} , {% endif %}
				{% endfor %}
			}`
var qtcFileContentTemplate = `
	{%% func %[1]s(d *%[2]s, mask FieldsMask) %%}
		%[3]s
	{%% endfunc %%}
`
