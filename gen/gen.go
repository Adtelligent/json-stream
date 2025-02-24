package gen

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/Adtelligent/json-stream/reg"
	"log"
	"regexp"
	"strings"
)

var copyFunctionsFeature = flag.Bool("copyFunctionsFeature", true, "Add copy function to structures")

type SrcFile struct {
	Content     []byte
	Structures  []string
	PackageName string
}

func (f *SrcFile) init() {
	for v := range reg.TypeRegistry {
		f.Structures = append(f.Structures, v)
	}
	f.readPackageName()
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
		if *copyFunctionsFeature {
			copyFunction, err := f.getCopyFromImplementation(className)
			if err != nil {
				return "", err
			}
			body.WriteString(generateCopyFunction(className, string(copyFunction)))

		}
		body.WriteString(generateMarshalJsonFile(className))
		t := reg.TypeRegistry[className]
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			entry := fmt.Sprintf("\t\"%[1]s.%[2]s\": reflect.TypeOf(%[1]s{}.%[2]s),", className, field.Name, className, field.Name)
			mapEntries = append(mapEntries, entry)
		}
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
	"bytes"
	"github.com/golang/protobuf/proto"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"io"
	"log"
	"reflect"
	"unsafe"
	"sync"
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

var sliceBytePool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0)
	},
}

func getSliceByte() []byte {
	return sliceBytePool.Get().([]byte)
}

func putSliceByte(slice []byte) {
	sliceBytePool.Put(slice[:0])
}
`

var getTypeMapMethodTemplate = `func GetType(path string) (reflect.Type, bool) {
	t, ok := fieldTypeMap[path]
	return t, ok
}
`
