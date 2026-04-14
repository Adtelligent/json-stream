package gen

import (
	"strings"
)

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
