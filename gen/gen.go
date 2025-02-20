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
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = log.Println
var _ = proto.Clone
var _ *structpb.Struct
var _ reflect.Type
var _ unsafe.Pointer


var DefaultFieldsRedefiner = &NoOpFieldRedefiner{}

type FieldRedefiner interface {
	 Redefine(path string, src unsafe.Pointer, dst unsafe.Pointer) bool
}

type NoOpFieldRedefiner struct {}

func (m *NoOpFieldRedefiner) Redefine(path string, src unsafe.Pointer, dst unsafe.Pointer) bool {
	return false
}
`

var getTypeMapMethodTemplate = `func GetType(path string) (reflect.Type, bool) {
	t, ok := fieldTypeMap[path]
	return t, ok
}
`
