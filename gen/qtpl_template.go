package gen

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
)

var (
	boolToInt             = flag.Bool("boolToInt", false, "bool to int generation")
	requiredFieldsConfig  = flag.String("requiredFields", "", "path to JSON config with required fields (used during code generation)")
	rawStringFieldsConfig = flag.String("rawStringFields", "", "path to JSON config with raw string fields (no escaping, only quotes)")
)

// Required fields are always output in JSON without empty value checks.
var requiredFieldsRegistry = make(map[string]map[string]struct{})

// Raw string fields are output with quotes but without escaping.
var rawStringFieldsRegistry = make(map[string]map[string]struct{})

// LoadRequiredFieldsConfig loads required fields configuration from JSON file.
// This is used during code generation to determine which fields should skip empty checks.
//
// JSON format: {"ClassName": ["Field1", "Field2"], ...}
//
// Example config.json:
//
//	{
//	  "BidRequest": ["Allimps"],
//	  "BidRequest_Imp_Video": ["Protocols", "Api"]
//	}
func LoadRequiredFieldsConfig(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read required fields config: %w", err)
	}

	var config map[string][]string
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse required fields config: %w", err)
	}

	for className, fields := range config {
		if requiredFieldsRegistry[className] == nil {
			requiredFieldsRegistry[className] = make(map[string]struct{})
		}
		for _, field := range fields {
			requiredFieldsRegistry[className][field] = struct{}{}
		}
	}

	return nil
}

// LoadRawStringFieldsConfig loads raw string fields configuration from JSON file.
// Raw string fields are output with quotes but without JSON escaping (for pre-formatted JSON strings).
//
// JSON format: {"ClassName.FieldName": true, ...}
//
// Example config.json:
//
//	{
//	  "BidResponse_SeatBid_Bid_Adm": ["Adm"]
//	}
func LoadRawStringFieldsConfig(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read raw string fields config: %w", err)
	}

	var config map[string][]string
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse raw string fields config: %w", err)
	}

	for className, fields := range config {
		if rawStringFieldsRegistry[className] == nil {
			rawStringFieldsRegistry[className] = make(map[string]struct{})
		}
		for _, field := range fields {
			rawStringFieldsRegistry[className][field] = struct{}{}
		}
	}

	return nil
}

func LoadRequiredFieldsIfConfigured() error {
	if requiredFieldsConfig != nil && *requiredFieldsConfig != "" {
		return LoadRequiredFieldsConfig(*requiredFieldsConfig)
	}
	return nil
}

func LoadRawStringFieldsIfConfigured() error {
	if rawStringFieldsConfig != nil && *rawStringFieldsConfig != "" {
		return LoadRawStringFieldsConfig(*rawStringFieldsConfig)
	}
	return nil
}

func isRequiredField(className string, fi FieldInfo) bool {
	if classFields, ok := requiredFieldsRegistry[className]; ok {
		_, isRequired := classFields[fi.Name]
		return isRequired
	}
	return false
}

func isRawStringField(className string, fi FieldInfo) bool {
	if classFields, ok := rawStringFieldsRegistry[className]; ok {
		_, isRaw := classFields[fi.Name]
		return isRaw
	}
	return false
}

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

func getJsonName(fi FieldInfo) string {
	jsonName := parseJSONNameFromProtoTag(fi.Tag.Get("protobuf"))
	if jsonName != "" {
		return jsonName
	}

	jsonName = parseNameFromProtoTag(fi.Tag.Get("protobuf"))
	if jsonName != "" {
		return jsonName
	}

	return strings.Replace(fi.Tag.Get("json"), ",omitempty", "", -1)
}

func getWriteJSON(className string, f *SrcFile) (string, error) {
	if f.isImplementator(className) {
		return getImplementatorJSON(className, f)
	} else {
		return getStructureJSON(className, f)
	}
}

func wrapTemplateWithCondition(template string, fi FieldInfo, className string) string {
	fieldName := fi.Name

	if isRequiredField(className, fi) {
		return template
	}

	var condition string
	switch fi.Kind {
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

func wrapTemplateWithConditionForImplementator(template string, fi FieldInfo, className string) string {
	fieldName := fi.Name
	condition := fmt.Sprintf("mask.In(\"%[1]s.%[2]s\")", className, fieldName)
	return fmt.Sprintf("{%% if %s %%}%s{%% endif %%}\n", condition, template)
}

func qtplFuncName(fi FieldInfo) string {
	// ElemType is set for pointer/slice fields (bare name, no package qualifier).
	// For struct fields (non-pointer), ElemType is empty; use TypeStr instead.
	name := fi.ElemType
	if name == "" {
		name = fi.TypeStr
	}
	return generateQtcName(getPrintableClassName(name))
}

func replaceMacros(template string, className string, fi FieldInfo) string {
	template = wrapTemplateWithCondition(template, fi, className)
	template = strings.Replace(template, "{fieldName}", fi.Name, -1)
	template = strings.Replace(template, "{className}", className, -1)
	template = strings.Replace(template, "{qtplFunc}", qtplFuncName(fi), -1)
	return template
}

func replaceMacrosForImplementator(template string, className string, fi FieldInfo) string {
	template = wrapTemplateWithConditionForImplementator(template, fi, className)
	template = strings.Replace(template, "{fieldName}", fi.Name, -1)
	template = strings.Replace(template, "{className}", className, -1)
	template = strings.Replace(template, "{qtplFunc}", qtplFuncName(fi), -1)
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
	si := f.Registry.Structs[className]
	var result bytes.Buffer
	result.WriteString("{\n")
	result.WriteString("		{% code comma := false %}\n")

	for _, fi := range si.Fields {
		jsonName := getJsonName(fi)

		if jsonName == "" && fi.Kind != reflect.Interface {
			continue
		}

		fieldTemplate, err := generateFieldTemplate(fi, f, jsonName, className)
		if err != nil {
			return "", fmt.Errorf("error generating template for field %s: %w", fi.Name, err)
		}

		result.WriteString(replaceMacros(fieldTemplate, className, fi))
	}

	result.WriteString("	}")

	return result.String(), nil
}

func getImplementatorJSON(className string, f *SrcFile) (string, error) {
	var result bytes.Buffer
	si := f.Registry.Structs[className]
	for _, fi := range si.Fields {
		jsonName := getJsonName(fi)

		if jsonName == "" && fi.Kind != reflect.Interface {
			continue
		}

		fieldTemplate, err := generateFieldTemplateForImplementator(fi, f, jsonName, className)
		if err != nil {
			return "", fmt.Errorf("error generating template for field %s: %w", fi.Name, err)
		}

		result.WriteString(replaceMacrosForImplementator(fieldTemplate, className, fi))
	}
	return result.String(), nil
}

func generateFieldTemplate(fi FieldInfo, f *SrcFile, jsonName string, className string) (string, error) {
	fieldName := formatFieldName(fi)
	template, err := generateInnerFieldTemplate(fi, fieldName, f, className)
	if err != nil {
		return "", err
	}
	wrappedTemplate := formatTemplate(jsonName, template)
	return wrappedTemplate, nil
}

func generateFieldTemplateForImplementator(fi FieldInfo, f *SrcFile, jsonName string, className string) (string, error) {
	fieldName := formatFieldName(fi)
	template, err := generateInnerFieldTemplate(fi, fieldName, f, className)
	if err != nil {
		return "", err
	}

	if fi.Kind == reflect.Ptr && jsonName != "" {
		fullTemplate := fmt.Sprintf("{%% if d.%s != nil %%}\"%s\":%s{%% endif %%}", fi.Name, jsonName, template)
		return fullTemplate, nil
	}

	wrappedTemplate := formatTemplateForImplementator(jsonName, template)
	return wrappedTemplate, nil
}

func formatFieldName(fi FieldInfo) string {
	if fi.Kind == reflect.Ptr {
		return "*d." + fi.Name
	} else if fi.Kind == reflect.Struct {
		return "&d." + fi.Name
	}
	return "d." + fi.Name
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

func generateInnerFieldTemplate(fi FieldInfo, fieldName string, f *SrcFile, className string) (string, error) {
	if fi.Kind == reflect.Interface {
		var bb bytes.Buffer
		implementators := f.findInterfaceImplementatorsByName(fi.TypeStr)
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
	switch fi.Kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return replaceTemplate(intQTPLFormatInnerTemplate, fieldName), nil
	case reflect.String:
		if isRawStringField(className, fi) {
			return replaceTemplate(rawStringQTPLFormatInnerTemplate, fieldName), nil
		}
		return replaceTemplate(stringQTPLFormatInnerTemplate, fieldName), nil
	case reflect.Bool:
		if *boolToInt {
			return replaceTemplate(boolToIntQTPLFormatInnerTemplate, fieldName), nil
		}
		return replaceTemplate(boolQTPLFormatInnerTemplate, fieldName), nil
	case reflect.Float32, reflect.Float64:
		return replaceTemplate(floatQTPLFormatInnerTemplate, fieldName), nil
	case reflect.Slice, reflect.Array:
		return generateSliceTemplate(fi, fieldName, f)
	case reflect.Struct:
		return generateStructTemplate(fi, fieldName, f)
	case reflect.Ptr:
		return generatePointerTemplate(fi, fieldName, f, className)
	case reflect.Map:
		return generateMapTemplate(fi, fieldName, f)
	default:
		return "", fmt.Errorf("unsupported type: %v", fi.Kind)
	}
}

func replaceTemplate(template, fieldName string) string {
	return strings.ReplaceAll(template, "{fieldName}", fieldName)
}

func generateSliceTemplate(fi FieldInfo, fieldName string, f *SrcFile) (string, error) {
	// Build a synthetic FieldInfo for the element type.
	elemTypeStr := fi.ElemType
	if elemTypeStr == "" {
		// fallback: strip leading []
		elemTypeStr = strings.TrimPrefix(fi.TypeStr, "[]")
		elemTypeStr = strings.TrimPrefix(elemTypeStr, "*")
	}
	rawElemTypeStr := strings.TrimPrefix(fi.TypeStr, "[]")
	elemKind, elemIsPtr, _, _, elemElem, _, _ := deriveKindWithRegistry(rawElemTypeStr, f.Registry.NamedTypes)
	elemFI := FieldInfo{
		Name:     "",
		TypeStr:  rawElemTypeStr,
		Kind:     elemKind,
		IsPtr:    elemIsPtr,
		ElemType: elemElem,
	}

	var nestedFieldName string
	if elemFI.Kind == reflect.Struct {
		nestedFieldName = "&v"
	} else {
		nestedFieldName = "v"
	}

	nestedTemplate, err := generateInnerFieldTemplate(elemFI, nestedFieldName, f, "")
	if err != nil {
		return "", err
	}

	totalVar := "total" + strings.ReplaceAll(fieldName, ".", "")
	template := replaceTemplate(sliceQTPLFormatInnerTemplate, fieldName)
	template = strings.ReplaceAll(template, "{totalVar}", totalVar)
	template = strings.ReplaceAll(template, "{nestedTemplate}", nestedTemplate)
	return template, nil
}

func generateStructTemplate(fi FieldInfo, fieldName string, f *SrcFile) (string, error) {
	fieldName = strings.ReplaceAll(fieldName, "*", "")
	// Check for structpb special types by full TypeStr
	typeStr := fi.TypeStr
	if typeStr == "structpb.Struct" || typeStr == "structpb.Value" {
		return replaceTemplate(structpbQTPLFormatTemplate, fieldName), nil
	}
	// Determine the struct type name to look up in the registry.
	// Priority: ElemType (populated for ptr/slice fields) > TypeStr (bare struct fields).
	// fi.Name is the Go field name — never use it as the struct type name.
	lookupName := fi.ElemType
	if lookupName == "" {
		// TypeStr may have a package qualifier; strip it for registry lookup.
		lookupName = typeStr
		if idx := strings.LastIndex(lookupName, "."); idx >= 0 {
			lookupName = lookupName[idx+1:]
		}
	}
	if _, ok := f.Registry.Structs[lookupName]; !ok {
		return "", fmt.Errorf("unknown struct %s", typeStr)
	}
	return replaceTemplate(structQTPLFormatInnerTemplate, fieldName), nil
}

func generatePointerTemplate(fi FieldInfo, fieldName string, f *SrcFile, className string) (string, error) {
	// Build FieldInfo for the element (dereferenced) type.
	elemTypeStr := fi.TypeStr
	if strings.HasPrefix(elemTypeStr, "*") {
		elemTypeStr = elemTypeStr[1:]
	}
	elemKind, elemIsPtr, elemIsSlice, elemIsMap, elemElem, elemMapKey, elemMapElem := deriveKindWithRegistry(elemTypeStr, f.Registry.NamedTypes)
	elemFI := FieldInfo{
		Name:     fi.Name,
		TypeStr:  elemTypeStr,
		Kind:     elemKind,
		IsPtr:    elemIsPtr,
		IsSlice:  elemIsSlice,
		IsMap:    elemIsMap,
		ElemType: elemElem,
		MapKey:   elemMapKey,
		MapElem:  elemMapElem,
	}

	nestedTemplate, err := generateInnerFieldTemplate(elemFI, fieldName, f, className)
	if err != nil {
		return "", err
	}
	result := strings.ReplaceAll(pointerQTPLFormatInnerTemplate, "{nestedTemplate}", nestedTemplate)
	if isRequiredField(className, fi) {
		ptrFieldName := strings.TrimPrefix(fieldName, "*")
		return fmt.Sprintf("{%% if %s != nil %%}%s{%% else %%}%s{%% endif %%}", ptrFieldName, result, zeroValueLiteral(elemFI)), nil
	}
	return result, nil
}

func zeroValueLiteral(fi FieldInfo) string {
	switch fi.Kind {
	case reflect.Bool:
		if *boolToInt {
			return "0"
		}
		return "false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "0"
	case reflect.String:
		return `""`
	default:
		return "null"
	}
}

func generateMapTemplate(fi FieldInfo, fieldName string, f *SrcFile) (string, error) {
	keyKind := deriveKindFromName(fi.MapKey)
	keyFI := FieldInfo{
		Name:    "",
		TypeStr: fi.MapKey,
		Kind:    keyKind,
	}

	var keyTemplate string
	var err error
	if keyFI.Kind == reflect.Struct {
		keyTemplate, err = generateInnerFieldTemplate(keyFI, "&k", f, "")
	} else {
		keyTemplate, err = generateInnerFieldTemplate(keyFI, "k", f, "")
	}
	if err != nil {
		return "", err
	}

	if keyFI.Kind != reflect.String {
		keyTemplate = `"` + keyTemplate + `"`
	}

	elemTypeStr := fi.MapElem
	elemKind, elemIsPtr, elemIsSlice, elemIsMap, elemElem, elemMapKey, elemMapElem := deriveKindWithRegistry(elemTypeStr, f.Registry.NamedTypes)
	valueFI := FieldInfo{
		Name:     "",
		TypeStr:  elemTypeStr,
		Kind:     elemKind,
		IsPtr:    elemIsPtr,
		IsSlice:  elemIsSlice,
		IsMap:    elemIsMap,
		ElemType: elemElem,
		MapKey:   elemMapKey,
		MapElem:  elemMapElem,
	}

	var valueTemplate string
	if valueFI.Kind == reflect.Struct {
		valueTemplate, err = generateInnerFieldTemplate(valueFI, "&v", f, "")
	} else {
		valueTemplate, err = generateInnerFieldTemplate(valueFI, "v", f, "")
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
var rawStringQTPLFormatInnerTemplate = `"{%s= {fieldName} %}"`

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
