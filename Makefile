DST ?= test/
SRC ?= /tmp/input.go

runWithCopyFeaturesFlagAndRequiredFields: preprocessing
	go run main.go -copyFunctionsFeature -requiredFields=$(REQ_FIELDS_CONFIG) -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

runWithCopyFeaturesFlag: preprocessing
	go run main.go -copyFunctionsFeature -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

runWithBoolToIntFlag: preprocessing
	go run main.go -boolToInt -- $(SRC) $(DST)
	$(GOPATH)/bin/qtc -dir=$(DST)

runWithFlags: preprocessing
	go run main.go -boolToInt -copyFunctionsFeature -requiredFields=$(REQ_FIELDS_CONFIG) -- $(SRC) $(DST)
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
