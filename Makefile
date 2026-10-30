APP     := moon_sweeper
OUT     := build
LDFLAGS := -s -w

# Without cgo, raylib-go embeds a prebuilt raylib for each desktop OS and loads
# it at runtime: every platform cross-compiles from any machine.
export CGO_ENABLED := 0

.PHONY: run build windows mac linux desktop clean

run:
	go run .

build:
	go build -ldflags "$(LDFLAGS)" -o $(OUT)/$(APP) .

windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS) -H=windowsgui" -o $(OUT)/windows-amd64/$(APP).exe .
	GOOS=windows GOARCH=arm64 go build -ldflags "$(LDFLAGS) -H=windowsgui" -o $(OUT)/windows-arm64/$(APP).exe .

# Universal binary (Apple Silicon + Intel). lipo only exists on macOS.
mac:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(OUT)/darwin-arm64/$(APP) .
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(OUT)/darwin-amd64/$(APP) .
	mkdir -p $(OUT)/macos
	lipo -create -output $(OUT)/macos/$(APP) $(OUT)/darwin-arm64/$(APP) $(OUT)/darwin-amd64/$(APP)

linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(OUT)/linux-amd64/$(APP) .
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(OUT)/linux-arm64/$(APP) .

desktop: windows mac linux

clean:
	rm -rf $(OUT)
