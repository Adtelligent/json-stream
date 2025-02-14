package main

import (
	"flag"
	"github.com/Adtelligent/json-stream/gen"
	"log"
	"os"
)

var prepocessing = flag.Bool("prepocessing", false, "is prepocessing mode")

func main() {
	flag.Parse()
	file := os.Args[len(os.Args)-3]
	importPath := os.Args[len(os.Args)-2]
	b, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("cant read file. err: %s", err)
	}

	if *prepocessing {
		gen.PreprocessFile(b, importPath)
		return
	}

	dstPath := os.Args[len(os.Args)-1]
	info, err := os.Stat(file)
	if err != nil {
		log.Fatalf("%s", err)
	}

	f := gen.NewWithContent(b, importPath)

	structuresFile, err := f.GetStructureFile()
	if err != nil {
		log.Fatalf("%s", err)
	}

	dstFileName := dstPath + info.Name() + ".gen.go"
	err = os.WriteFile(dstFileName, []byte(structuresFile), os.ModePerm)
	if err != nil {
		log.Fatalf("%s", err)
	}

	qb, err := f.GetQTPLFile()
	qtplDstFileName := dstPath + info.Name() + ".gen.qtpl"
	err = os.WriteFile(qtplDstFileName, []byte(qb), os.ModePerm)
	if err != nil {
		log.Fatalf("%s", err)
	}
}
