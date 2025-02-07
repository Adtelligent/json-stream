DST=test/
IMPORT_PATH=json-stream/test
SRC=test/test.go

run: preprocessing
	go run main.go -- $(SRC) $(IMPORT_PATH) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)
preprocessing: clear
	go run main.go -prepocessing $(SRC) $(IMPORT_PATH) $(DST)

clear:
	rm -f $(DST)*.gen.go
	rm -f $(DST)*.gen.qtpl.go
	rm -f $(DST)*.gen.qtpl
	rm -f reg/preproc.go
