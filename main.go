package main

import (
	"flag"
	"github.com/Adtelligent/json-stream/gen"
	"log"
	"os"
	"path/filepath"
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

	qb, err := f.GetQTPLFile()
	if err != nil {
		log.Fatalf("failed to get QTPL file. err: %s", err)
	}
	qtplDstFileName := dstPath + info.Name() + ".gen.qtpl"
	err = os.WriteFile(qtplDstFileName, []byte(qb), os.ModePerm)
	if err != nil {
		log.Fatalf("failed to write generated qtpl file. err: %s", err)
	}
	err = gen.RemovePreprocessFiles()
	if err != nil {
		log.Fatalf("failed to remove preprocess files. err: %s", err)
	}
}

func readCombinedContent(path string) ([]byte, os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	var content []byte
	if info.IsDir() {
		firstFile := true
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
			if !firstFile {
				fileContent = gen.RemovePackageDeclaration(fileContent)
			} else {
				firstFile = false
			}
			content = append(content, []byte(fileContent)...)
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	} else {
		content, err = os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
	}
	return content, info, nil
}
