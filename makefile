build:
	GOOS=darwin GOARCH=arm64 go build -o warp cmd/*
	chmod +x warp