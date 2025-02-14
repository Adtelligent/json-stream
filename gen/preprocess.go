package gen

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

const outPath = "result/copy.go"

var structReg = regexp.MustCompile(`type ([A-Z]\w*) struct`)
var packageReg = regexp.MustCompile(`package\s+(\w+)`)
var fileTemplate = `
package reg

import (
	"github.com/Adtelligent/json-stream/result"
)

func init() {
	{result}
}
`

func PreprocessFile(content []byte) {
	var res []string
	for _, match := range structReg.FindAllSubmatch(content, -1) {
		structName := string(match[1])
		res = append(res, fmt.Sprintf(`registerType((*%s.%s)(nil))`, "result", structName))
	}

	finalResult := strings.Replace(fileTemplate, "{result}", strings.Join(res, "\n\t"), -1)

	err := os.WriteFile("reg/preproc.go", []byte(finalResult), os.ModePerm)
	if err != nil {
		log.Fatalf("%s", err)
	}
}

func ChangeInputFilePackageAndSave(filePath []byte) error {
	newContent := packageReg.ReplaceAll(filePath, []byte("package result"))
	if err := os.WriteFile(outPath, newContent, 0644); err != nil {
		return fmt.Errorf("failed to write file to %s: %w", outPath, err)
	}

	return nil
}
