package gen

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

var structReg = regexp.MustCompile(`type ([A-Z]\w*) struct`)
var packageReg = regexp.MustCompile(`package\s+(\w+)`)
var fileTemplate = `
package reg

import (
	"{importPath}"
)

func init() {
	{result}
}
`

func PreprocessFile(content []byte, importPath string) {
	packageMatch := packageReg.FindSubmatch(content)
	if len(packageMatch) < 2 {
		log.Fatalf("Unable to determine package name")
	}
	currentPackage := string(packageMatch[1])

	var res []string
	for _, match := range structReg.FindAllSubmatch(content, -1) {
		structName := string(match[1])
		res = append(res, fmt.Sprintf(`registerType((*%s.%s)(nil))`, currentPackage, structName))
	}

	finalResult := strings.Replace(fileTemplate, "{importPath}", importPath, -1)
	finalResult = strings.Replace(finalResult, "{result}", strings.Join(res, "\n\t"), -1)

	err := os.WriteFile("reg/preproc.go", []byte(finalResult), os.ModePerm)
	if err != nil {
		log.Fatalf("%s", err)
	}
}
