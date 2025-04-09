package gen

import (
	"bytes"
	"github.com/Adtelligent/json-stream/reg"
	"reflect"
	"strings"
)

func (f *SrcFile) getUnmarshalFile() (string, error) {
	var buf bytes.Buffer
	header := strings.Replace(packageTemplate, "{packageName}", f.PackageName, -1)
	header = strings.Replace(header, "{customCode}", customCodeTemplate, -1)
	buf.WriteString(header)
	updateBody := ""
	for _, structName := range f.Structures {
		strType := reg.TypeRegistry[structName]
		for i := 0; i < strType.NumField(); i++ {
			field := strType.Field(i)
			if !field.IsExported() || field.Type.Kind() != reflect.Interface {
				continue
			}
			bindingBlocks := ""
			implementators := f.findInterfaceImplementators(field.Type)
			for _, impl := range implementators {
				implType := reg.TypeRegistry[impl]
				jsonName := getJsonName(implType.Field(0))
				decoderName := "myInterfaceFieldDecoder" + impl + "_"
				block := strings.Replace(bindingBlockTemplate, "{fieldName}", field.Name, -1)
				block = strings.Replace(block, "{impl}", impl, -1)
				block = strings.Replace(block, "{jsonName}", jsonName, -1)
				block = strings.Replace(block, "{decoderName}", decoderName, -1)
				bindingBlocks += block
			}
			fieldBlock := strings.Replace(updateFieldTemplate, "{fullStructName}", f.PackageName+"."+structName, -1)
			fieldBlock = strings.Replace(fieldBlock, "{fieldName}", field.Name, -1)
			fieldBlock = strings.Replace(fieldBlock, "{bindingBlocks}", bindingBlocks, -1)
			updateBody += fieldBlock
		}
	}
	customExtensionSection := strings.Replace(customExtensionTemplate, "{updateBody}", updateBody, -1)
	buf.WriteString(customExtensionSection)

	for _, structName := range f.Structures {
		strType := reg.TypeRegistry[structName]
		for i := 0; i < strType.NumField(); i++ {
			field := strType.Field(i)
			if !field.IsExported() || field.Type.Kind() != reflect.Interface {
				continue
			}
			implementators := f.findInterfaceImplementators(field.Type)
			for _, impl := range implementators {
				implType := reg.TypeRegistry[impl]
				decoderName := "myInterfaceFieldDecoder" + impl + "_"
				block := strings.Replace(decoderTemplate, "{decoderName}", decoderName, -1)
				block = strings.Replace(block, "{impl}", impl, -1)
				block = strings.Replace(block, "{firstFieldName}", implType.Field(0).Name, -1)
				block = strings.Replace(block, "{interfaceType}", field.Type.Name(), -1)
				buf.WriteString(block)
			}
		}
	}

	buf.WriteString(extensionMethodsTemplate)

	return buf.String(), nil
}

var packageTemplate = `package {packageName}

import (
	"fmt"
	"unsafe"
	"reflect"
	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

{customCode}
`
var customCodeTemplate = `type customBoolDecoder struct{}

func init() {
	json.RegisterExtension(&customExtension{})
	jsoniter.RegisterTypeDecoder("bool", &customBoolDecoder{})
}

func UnmarshalJson(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
`
var extensionMethodsTemplate = `func (ext *customExtension) CreateMapKeyDecoder(typ reflect2.Type) jsoniter.ValDecoder { return nil }
func (ext *customExtension) CreateMapKeyEncoder(typ reflect2.Type) jsoniter.ValEncoder { return nil }
func (ext *customExtension) CreateDecoder(typ reflect2.Type) jsoniter.ValDecoder { return nil }
func (ext *customExtension) CreateEncoder(typ reflect2.Type) jsoniter.ValEncoder { return nil }
func (ext *customExtension) DecorateDecoder(typ reflect2.Type, decoder jsoniter.ValDecoder) jsoniter.ValDecoder {
	return decoder
}
func (ext *customExtension) DecorateEncoder(typ reflect2.Type, encoder jsoniter.ValEncoder) jsoniter.ValEncoder {
	return encoder
}

func (d *customBoolDecoder) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	switch iter.WhatIsNext() {
	case jsoniter.NumberValue:
		num := iter.ReadInt64()
		var b bool
		switch num {
		case 0:
			b = false
		case 1:
			b = true
		default:
			iter.ReportError("customBoolDecoder", fmt.Sprintf("invalid numeric value: %d", num))
			return
		}
		*(*bool)(ptr) = b
	case jsoniter.BoolValue:
		*(*bool)(ptr) = iter.ReadBool()
	default:
		iter.ReportError("customBoolDecoder", "unexpected type for bool")
	}
}
`
var decoderTemplate = `type {decoderName} struct {
	defaultDecoder jsoniter.ValDecoder
	fieldName string
}

func (d {decoderName}) GetFieldName() string { return d.fieldName }

func (d {decoderName}) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	instance := new({impl})
	iter.ReadVal(&instance.{firstFieldName})
	*((*{interfaceType})(ptr)) = instance
}
`
var customExtensionTemplate = `type customExtension struct{}

func (ext *customExtension) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
{updateBody}
}
`
var bindingBlockTemplate = `				binding_{fieldName}_{impl} := &jsoniter.Binding{
					Field: field.Field,
					FromNames: []string{"{jsonName}"},
					ToNames: []string{"{jsonName}"},
					Encoder: field.Encoder,
					Decoder: {decoderName}{defaultDecoder: field.Decoder, fieldName: "{jsonName}"},
				}
				{
					rv := reflect.ValueOf(binding_{fieldName}_{impl}).Elem()
					lf := rv.FieldByName("levels")
					reflect.NewAt(lf.Type(), unsafe.Pointer(lf.UnsafeAddr())).Elem().Set(reflect.ValueOf([]int{baseLevel}))
				}
				structDescriptor.Fields = append(structDescriptor.Fields, binding_{fieldName}_{impl})
				baseLevel++
`
var updateFieldTemplate = `	if structDescriptor.Type.String() == "{fullStructName}" {
		for _, field := range structDescriptor.Fields {
			if field.Field.Name() == "{fieldName}" {
				baseLevel := len(structDescriptor.Fields) + 1
{bindingBlocks}
			}
		}
	}
`
