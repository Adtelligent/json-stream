DST ?= test/
SRC ?= /tmp/input.go

runBoolToInt: preprocessing
	go run main.go -boolToInt $(SRC) $(DST)
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
	rm -f reg/preproc.go
	rm -f result/copy.go
