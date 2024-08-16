build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o warp cmd/*
	chmod +x warp

build:
	go build -o warp cmd/*
	chmod +x warp