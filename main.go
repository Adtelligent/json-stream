package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/Adtelligent/json-stream/gen"
)

var prepocessing = flag.Bool("prepocessing", false, "is prepocessing mode")

func main() {
	flag.Parse()
	args := os.Args
	if len(args) < 3 {
		log.Fatal("Usage: <program> [options] <source_path> <destination_path>")
	}

	sourcePath := args[len(args)-2]
	dstPath := args[len(args)-1]

	if err := gen.LoadRequiredFieldsIfConfigured(); err != nil {
		log.Fatalf("failed to load required fields config: %s", err)
	}

	if err := gen.LoadRawStringFieldsIfConfigured(); err != nil {
		log.Fatalf("failed to load raw string fields config: %s", err)
	}

	b, info, err := readCombinedContent(sourcePath)
	if err != nil {
		log.Fatalf("failed to read source content: %s", err)
	}
	if err := gen.ChangeInputFilePackageAndSave(b); err != nil {
		log.Fatalf("cant read file. err: %s", err)
	}
	if *prepocessing {
		gen.PreprocessFile(b)
		return
	}

	defer func() {
		err := gen.RemovePreprocessFiles()
		if err != nil {
			log.Fatalf("failed to remove preprocess files. err: %s", err)
		}
	}()

	f := gen.NewWithContent(b)

	structuresFile, err := f.GetStructureFile()
	if err != nil {
		log.Fatalf("failed to get structure file. err: %s", err)
	}

	dstFileName := dstPath + info.Name() + ".gen.go"
	err = os.WriteFile(dstFileName, []byte(structuresFile), os.ModePerm)
	if err != nil {
		log.Fatalf("failed to write generated go file. err: %s", err)
	}

	unmarshallingFile, err := f.GetUnmarshalFile()
	if err != nil {
		log.Fatalf("failed to get structure file. err: %s", err)
	}

	unmarshallingDstFileName := dstPath + "unmarshalling.gen.go"
	err = os.WriteFile(unmarshallingDstFileName, []byte(unmarshallingFile), os.ModePerm)
	if err != nil {
		log.Fatalf("failed to write generated go file. err: %s", err)
	}

	qb, err := f.GetQTPLFile()
	if err != nil {
		log.Fatalf("failed to get QTPL file. err: %s", err)
	}
	qtplDstFileName := dstPath + info.Name() + ".gen.qtpl"
	err = os.WriteFile(qtplDstFileName, []byte(qb), os.ModePerm)
	if err != nil {
		log.Fatalf("failed to write generated qtpl file. err: %s", err)
	}
}

func readCombinedContent(path string) ([]byte, os.FileInfo, error) {
	if containsComma(path) {
		return readMultipleFiles(path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	var content []byte
	if info.IsDir() {
		var allImports []string
		var packageName string
		var bodies []string
		importSet := make(map[string]struct{})

		err := filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.IsDir() || filepath.Ext(p) != ".go" {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			fileContent := string(data)

			if packageName == "" {
				packageName = extractPackageName(fileContent)
			}

			imports := gen.ExtractImports(fileContent)
			for _, imp := range imports {
				if _, exists := importSet[imp]; !exists {
					importSet[imp] = struct{}{}
					allImports = append(allImports, imp)
				}
			}

			body := gen.RemovePackageAndImports(fileContent)
			bodies = append(bodies, body)
			return nil
		})
		if err != nil {
			return nil, nil, err
		}

		var result string
		result += "package " + packageName + "\n\n"
		if len(allImports) > 0 {
			result += "import (\n"
			for _, imp := range allImports {
				result += "\t" + imp + "\n"
			}
			result += ")\n"
		}
		for _, body := range bodies {
			result += body
		}
		content = []byte(result)
	} else {
		content, err = os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
	}
	return content, info, nil
}

func readMultipleFiles(paths string) ([]byte, os.FileInfo, error) {
	files := splitByComma(paths)
	if len(files) == 0 {
		return nil, nil, os.ErrNotExist
	}

	var allImports []string
	var packageName string
	var bodies []string
	importSet := make(map[string]struct{})
	var firstInfo os.FileInfo

	for i, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			firstInfo = info
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return nil, nil, err
		}
		fileContent := string(data)

		if packageName == "" {
			packageName = extractPackageName(fileContent)
		}

		imports := gen.ExtractImports(fileContent)
		for _, imp := range imports {
			if _, exists := importSet[imp]; !exists {
				importSet[imp] = struct{}{}
				allImports = append(allImports, imp)
			}
		}

		body := gen.RemovePackageAndImports(fileContent)
		bodies = append(bodies, body)
	}

	var result string
	result += "package " + packageName + "\n\n"
	if len(allImports) > 0 {
		result += "import (\n"
		for _, imp := range allImports {
			result += "\t" + imp + "\n"
		}
		result += ")\n"
	}
	for _, body := range bodies {
		result += body
	}

	return []byte(result), firstInfo, nil
}

func splitByComma(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if i > start {
				result = append(result, trim(s[start:i]))
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, trim(s[start:]))
	}
	return result
}

func containsComma(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return true
		}
	}
	return false
}

func extractPackageName(content string) string {
	for _, line := range splitLines(content) {
		trimmed := trim(line)
		if hasPrefix(trimmed, "package ") {
			return trimmed[8:]
		}
	}
	return ""
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trim(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
