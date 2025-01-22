module qtpl-generator

go 1.22.8

toolchain go1.23.1

replace github.com/valyala/fasthttp => github.com/aradilov/fasthttp v1.44.1-0.20240430190254-17004aa8d771

require (
	github.com/golang/protobuf v1.5.4
	github.com/valyala/quicktemplate v1.8.0
	gitlab.adtelligent.com/common/shared v0.0.0-20241128083343-6d219f1fb748
	gitlab.adtelligent.com/videe/rev-share-backend v0.0.0-20250121085000-bba2065edb00
	google.golang.org/protobuf v1.36.3
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/valyala/batcher v0.0.0-20161116154623-b113a4d4f9d9 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.57.0 // indirect
	github.com/vharitonsky/iniflags v0.0.0-20180513140207-a33cd0b5f3de // indirect
)
