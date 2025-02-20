DST ?= test/
SRC ?= /tmp/input.go

runWithFlags: preprocessing
	go run main.go -boolToInt -copyFunctionsFeature $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

run: preprocessing
	go run main.go -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

preprocessing: clear
	go run main.go -prepocessing $(SRC) $(DST)

clear:
	rm -f $(DST)*.gen.go
	rm -f $(DST)*.gen.qtpl.go
	rm -f $(DST)*.gen.qtpl
