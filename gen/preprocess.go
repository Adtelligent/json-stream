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

func RemovePreprocessFiles() error {
	if err := os.Remove(preprocessCopyPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(inputFileCopyPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = os.Remove(copyDir)
	return nil
}

func RemovePackageAndImports(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inImportBlock := false
	skipNext := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "package ") {
			skipNext = true
			continue
		}

		if skipNext && trimmed == "" {
			continue
		}
		skipNext = false

		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
			continue
		}

		if strings.HasPrefix(trimmed, "import ") && !strings.Contains(trimmed, "(") {
			continue
		}

		if inImportBlock {
			if trimmed == ")" {
				inImportBlock = false
			}
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func ExtractImports(content string) []string {
	var imports []string
	lines := strings.Split(content, "\n")
	inImportBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
			continue
		}

		if strings.HasPrefix(trimmed, "import ") && !strings.Contains(trimmed, "(") {
			imp := strings.TrimPrefix(trimmed, "import ")
			imports = append(imports, imp)
			continue
		}

		if inImportBlock {
			if trimmed == ")" {
				inImportBlock = false
				continue
			}
			if trimmed != "" {
				imports = append(imports, trimmed)
			}
		}
	}

	return imports
}
