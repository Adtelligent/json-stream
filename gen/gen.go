package gen

import (
	"bytes"
	"gitlab.adtelligent.com/common/shared/log"
	"qtpl-generator/reg"
	"regexp"
	"strings"
)

type SrcFile struct {
	Content     []byte
	Structures  []string
	PackageName string
	ImportPath  string
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
	for _, str := range f.Structures {
		copyFrom, err := f.getCopyFromImplementation(str)
		if err != nil {
			return "", err
		}
		result.WriteString(generateStructureFile(str, string(copyFrom)))
	}

	header := strings.Replace(structureFileTemplate, "{packageName}", f.PackageName, 1)

	return header + "\n" + result.String(), nil
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

func NewWithContent(b []byte, importPath string) *SrcFile {
	f := &SrcFile{
		Content:    b,
		ImportPath: importPath,
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
	"log"
	"github.com/golang/protobuf/proto"
	structpb "github.com/golang/protobuf/ptypes/struct"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = log.Println
var _ = proto.Clone
var _ *structpb.Struct
`
