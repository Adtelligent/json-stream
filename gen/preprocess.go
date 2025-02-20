package gen

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

const copyDir = "copy"
const inputFileCopyPath = "copy/copy.go"
const preprocessCopyPath = "reg/preproc.go"

var structReg = regexp.MustCompile(`type ([A-Z]\w*) struct`)
var packageReg = regexp.MustCompile(`package\s+(\w+)`)
var fileTemplate = `
package reg

import (
	"github.com/Adtelligent/json-stream/copy"
)

func init() {
	{result}
}
`

func PreprocessFile(content []byte) {
	var res []string
	for _, match := range structReg.FindAllSubmatch(content, -1) {
		structName := string(match[1])
		res = append(res, fmt.Sprintf(`registerType((*%s.%s)(nil))`, copyDir, structName))
	}

	finalResult := strings.Replace(fileTemplate, "{result}", strings.Join(res, "\n\t"), -1)

	err := os.WriteFile(preprocessCopyPath, []byte(finalResult), os.ModePerm)
	if err != nil {
		log.Fatalf("%s", err)
	}
}

func ChangeInputFilePackageAndSave(filePath []byte) error {
	newContent := packageReg.ReplaceAll(filePath, []byte("package copy"))

	if err := os.MkdirAll(copyDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory 'result': %w", err)
	}

	if err := os.WriteFile(inputFileCopyPath, newContent, 0644); err != nil {
		return fmt.Errorf("failed to write file to %s: %w", inputFileCopyPath, err)
	}

	return nil
}

func RemovePackageDeclaration(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			lines = append(lines[:i], lines[i+1:]...)
			break
		}
	}
	return strings.Join(lines, "\n")
}
