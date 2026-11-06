.PHONY: dev build build-pi test css

dev:
	@$(MAKE) -j2 watch-css watch-go

watch-css:
	tailwindcss -i assets/css/input.css -o assets/css/index.css --watch

watch-go:
	air

css:
	tailwindcss -i assets/css/input.css -o assets/css/index.css

build:
	go build -o bin/flashcards .

# Linux/arm64 binary for the Raspberry Pi (CGO off: pure-Go SQLite).
build-pi:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/flashcards-arm64 .

test:
	go test ./...
