.PHONY: dev build build-pi test css apk apk-run apk-logs apk-go-log apk-push-db apk-clean

# Task runner dependency — no global install required:
#   Go 21+ and the repo's go.mod pin go-task, so `go run` resolves it locally
#   (after the normal `go mod download` any Go build already needs).
TASK := go run github.com/go-task/task/v3/cmd/task

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

# ---------- Android (device build + install; requires toolchain setup, see README) ----------

apk:
	@$(TASK) apk:build

apk-run:
	@$(TASK) apk:run:device

apk-logs:
	@$(TASK) apk:logs

apk-go-log:
	@$(TASK) apk:go-log

apk-push-db:
	@$(TASK) apk:push:db

apk-clean:
	@$(TASK) clean
