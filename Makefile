DST ?= test/
SRC ?= /tmp/input.go

runWithAllFlags: clear
	go run main.go -copyFunctionsFeature -requiredFields=$(REQ_FIELDS_CONFIG) -rawStringFields=$(RAW_STRING_FIELDS_CONFIG) -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

runWithAllFlagsAndBoolToInt: clear
	go run main.go -boolToInt -copyFunctionsFeature -requiredFields=$(REQ_FIELDS_CONFIG) -rawStringFields=$(RAW_STRING_FIELDS_CONFIG) -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

runWithCopyFeaturesFlag: clear
	go run main.go -copyFunctionsFeature -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

runWithBoolToIntFlag: clear
	go run main.go -boolToInt -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

runWithFlags: clear
	go run main.go -boolToInt -copyFunctionsFeature -requiredFields=$(REQ_FIELDS_CONFIG) -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

runWithRawStringFields: clear
	go run main.go -rawStringFields=$(RAW_STRING_FIELDS_CONFIG) -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

run: clear
	go run main.go -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

clear:
	rm -f $(DST)*.gen.go
	rm -f $(DST)*.gen.qtpl.go
	rm -f $(DST)*.gen.qtpl
