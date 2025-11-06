package gen

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"

	"github.com/Adtelligent/json-stream/reg"
)

var copyFunctionsFeature = flag.Bool("copyFunctionsFeature", true, "Add copy function to structures")

type SrcFile struct {
	Content                 []byte
	Structures              []string
	PackageName             string
	Implementators          map[string]struct{}
	ImplementatorStructures map[string]struct{}
}

func (f *SrcFile) init() {
	for v := range reg.TypeRegistry {
		f.Structures = append(f.Structures, v)
	}
	f.readPackageName()
	f.findAllImplementators()
}

var packageNameReg = regexp.MustCompile(`package\s+(\w+)`)

func (f *SrcFile) readPackageName() {
	packageName := packageNameReg.FindAllSubmatch(f.Content, 1)
	if len(packageName) == 0 {
		log.Fatalf("cant determine package name")
	}

	f.PackageName = string(packageName[0][1])
}

func (f *SrcFile) GetStructureFile() (string, error) {
	var result bytes.Buffer
	var body bytes.Buffer
	var mapEntries []string
	for _, className := range f.Structures {
		strType := reg.TypeRegistry[className]
		if *copyFunctionsFeature {
			copyFunction, err := f.getCopyFromImplementation(className, strType)
			if err != nil {
				return "", err
			}
			if f.isImplementator(className) {
				body.WriteString(generateCopyFunctionForImplementator(className, string(copyFunction)))
			} else {
				body.WriteString(generateCopyFunction(className, string(copyFunction)))
			}

		}
		body.WriteString(generateMarshalJsonFile(className))
		for i := 0; i < strType.NumField(); i++ {
			field := strType.Field(i)
			if !field.IsExported() {
				continue
			}
			entry := fmt.Sprintf("\t\"%[1]s.%[2]s\": reflect.TypeOf(%[1]s{}.%[2]s),", className, field.Name, className, field.Name)
			mapEntries = append(mapEntries, entry)
		}

		body.WriteString(generateGetValueUnsafePointerMethod(className, strType))

	}

	result.WriteString(strings.Replace(structureFileTemplate, "{packageName}", f.PackageName, 1))
	mapCode := "\nvar fieldTypeMap = map[string]reflect.Type{\n" +
		strings.Join(mapEntries, "\n") +
		"\n}\n"
	result.WriteString(mapCode)
	result.WriteString(getTypeMapMethodTemplate)
	result.Write(body.Bytes())
	return result.String(), nil
}

func (f *SrcFile) GetUnmarshalFile() (string, error) {
	return f.getUnmarshalFile()
}

func (f *SrcFile) findInterfaceImplementators(interfaceType reflect.Type) []string {
	result := []string{}
	for _, s := range f.Structures {
		if reflect.PointerTo(reg.TypeRegistry[s]).Implements(interfaceType) {
			result = append(result, s)
		}
	}

	return result
}

func (f *SrcFile) findAllImplementators() {
	f.Implementators = make(map[string]struct{})
	f.ImplementatorStructures = make(map[string]struct{})

	for _, v := range f.Structures {
		totalFields := reg.TypeRegistry[v].NumField()
		for i := 0; i < totalFields; i++ {
			field := reg.TypeRegistry[v].Field(i)
			if field.Type.Kind() == reflect.Interface {
				for _, impl := range f.findInterfaceImplementators(field.Type) {
					f.Implementators[impl] = struct{}{}
					t := reg.TypeRegistry[impl].Field(0).Type.Name()
					f.ImplementatorStructures[t] = struct{}{}
				}
			}
		}
	}
}

func (f *SrcFile) isImplementator(className string) bool {
	_, ok := f.Implementators[className]
	return ok
}

func (f *SrcFile) GetQTPLFile() (string, error) {
	var bb bytes.Buffer
	for _, sName := range f.Structures {
		qb, err := GetQTPLFile(sName, f)
		if err != nil {
			log.Fatalf("%s", err)
		}

		bb.WriteString(qb)
	}

	return strings.Replace(qtcFileTemplate, "{content}", bb.String(), -1), nil
}

func NewWithContent(b []byte) *SrcFile {
	f := &SrcFile{
		Content: b,
	}
	f.init()

	return f
}

var qtcFileTemplate = `{% stripspace %}
{% import

"log"

%}

{%code 
	var _ = log.Printf
%}

{content}

{% endstripspace %}
`
var structureFileTemplate = `
package {packageName}

import (
	"flag"
	"bytes"
	"fmt"
	"github.com/golang/protobuf/proto"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"io"
	"log"
	"reflect"
	"strconv"
	"sync"
	"unsafe"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = log.Println
var _ = proto.Clone
var _ *structpb.Struct
var _ unsafe.Pointer
var _ sync.Pool


var DefaultFieldsRedefiner = &NoOpFieldRedefiner{}

// FieldRedefiner is an interface that redefines a field's value.
// Parameters:
// - typ: A type string, for example "BidRequest_Device_UserAgent.Browsers".
//        It encodes both the structure name and the field name.(see GetType)
// - path: The path from the main object to the target object, e.g., "BidRequest.Device.Sua".
// - field: The field name within the structure, e.g., "Browsers".
// - src: Pointer to the source value.
// - dst: Pointer to the destination value.
// The Redefine method should return true if the redefinition was successful; if false, dst will simply be set to src.
type FieldRedefiner interface {
	 Redefine(typ string, path, field []byte, src unsafe.Pointer, dst unsafe.Pointer) bool
}

type NoOpFieldRedefiner struct {}

func (m *NoOpFieldRedefiner) Redefine(typ string, path, field []byte, src unsafe.Pointer, dst unsafe.Pointer) bool {
	return false
}

var capSliceBytePool = flag.Uint("capSliceBytePool", 1024, "Capacity of slice byte for pool")

var sliceBytePool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, *capSliceBytePool)
	},
}

func getSliceByte() []byte {
	return sliceBytePool.Get().([]byte)
}

func putSliceByte(slice []byte) {
	sliceBytePool.Put(slice[:0])
}

var FieldsMasksZero *FieldsLimiter

// FieldsMask is an interface that controls which fields are included during JSON marshaling.
// Parameters:
// - path: A path string identifying the field, e.g., "BidRequest.Device.Ip".
//         The format follows the structure: "{StructName}.{FieldName}" or nested paths.
// The In method should return true if the field should be included in the output;
// if false, the field will be omitted from the JSON.
type FieldsMask interface {
	In(path string) bool
}

type FieldsLimiter struct {
	Fields map[string]struct{}
}

func (m *FieldsLimiter) In(path string) bool {
	if m == nil {
		return true
	}

	_, ok := m.Fields[path]
	return ok
}

type FieldsScanner struct {
	Fields map[string]struct{}
}

func (m *FieldsScanner) In(path string) bool {
	if m.Fields == nil {
		m.Fields = make(map[string]struct{})
	}
	m.Fields[path] = struct{}{}
	return true
}
`

var getTypeMapMethodTemplate = `func GetType(path string) reflect.Type {
	return fieldTypeMap[path]
}
`
