# Json-stream Marshaling Library

This library provides a fast and efficient solution for JSON marshaling in Golang through code generation and quicktemplate templates. It is particularly effective for structures with a large number of fields, ensuring optimal performance and reduced overhead compared to traditional JSON encoding approaches.
## Features
- **Code generation for JSON marshaling**
- Support for various field types, including integers, strings, booleans, floats, slices, maps, structs, and pointers
- Custom handling for `structpb.Struct`
- Optional preprocessing mode for advanced control
- Ability to generate copy functions for structures with customizable field selection:
  - The generated copy functions accept a limiter (FieldsLimiter interface) that allows you to specify which fields should be copied.
- Flag boolToInt to automatically generate conversion logic for boolean types into integers

## Getting Started

### Prerequisites
- `qtc` binary from [quicktemplate](https://github.com/valyala/quicktemplate)

### Running the Generator
1. Configure the Makefile with appropriate paths:
    - `DST`: The output directory for generated files
    - `SRC`: The source file or directory containing Go files with structures to be marshaled. **Note:** SRC can be either a single file with structures or a directory with multiple Go files.

2. Run the generator:
The Makefile defines several targets:

**Default Run**
```bash
make run
```
This target performs preprocessing and then runs the generator with default settings:
   - It calls `go run main.go -- $(SRC) $(DST)`
   - Finally, it invokes qtc to process the generated templates.

**Run with Flags**

```bash
make runWithFlags
```
This target enables additional functionality by passing the new flags:
   - `-boolToInt`: Generates conversion logic for boolean values to integer.
   - `-copyFunctionsFeature`: Generates copy functions for structures. These functions receive a limiter to determine which fields to copy.

It also runs preprocessing before generating code and finally calls qtc.

**Preprocessing**

```bash
make preprocessing
```
This target performs any necessary preprocessing steps:
```makefile
preprocessing: clear
	go run main.go -prepocessing $(SRC) $(DST)
```

**Clear**

```bash
make clear
```
This target removes generated files:
```makefile
clear:
	rm -f $(DST)*.gen.go
	rm -f $(DST)*.gen.qtpl.go
	rm -f $(DST)*.gen.qtpl
```

### Flags
The generator supports the following flags:
- `-prepocessing` (boolean): Enable preprocessing mode for additional setup before code generation.
- `-boolToInt` (boolean): Enable the conversion of boolean types to integers in the generated code.
- `-copyFunctionsFeature` (boolean): Generate copy functions for structures. These functions accept a limiter—an implementation of the `FieldsLimiter` interface—that determines which fields should be copied.

### Customizing Field Copying
You can control which fields are copied by implementing the `FieldsLimiter` interface. The default implementation is shown below:
```go
var DefaultFieldsLimiter = &NoOpFieldsLimiter{}

type FieldsLimiter interface {
	In(path string) bool
}

type NoOpFieldsLimiter struct {
	Fields map[string]struct{}
}

func (m *NoOpFieldsLimiter) In(path string) bool {
	return true
}
```
Implement your own version of FieldsLimiter if you need to set conditions on which fields are copied.

### Supported Field Types
The library supports JSON marshaling for the following field types:
- **Integers**: `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- **Strings**
- **Booleans**
- **Floats**: `float32`, `float64`
- **Slices and arrays**
- **Maps**
- **Structs**
- **Pointers**

Additional handling for `structpb.Struct` is included.

# Benchmarks Marshaling
The benchmark was conducted using a data structure based on the [OpenRTB protocol](https://developers.google.com/authorized-buyers/rtb/downloads/openrtb-proto) from Google.
![benchmark.png](benchmarks/benchmark.png)

## Contributing
Contributions are welcome! Feel free to open issues or submit pull requests.

---
For more details, check out the source code or contact the maintainer.