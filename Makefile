BINARY  := guitar-coach
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/luca-filipponi/guitar-coach/internal/cli.version=$(VERSION)
DEV_DIR := .dev-data
DEV_PORT ?= 8080

.PHONY: build run dev dev-stop dev-clean test vet fmt clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

run:
	go run .

# dev: rebuild and relaunch the web UI against a throwaway demo data dir.
dev: build
	@-./bin/$(BINARY) --dir $(DEV_DIR) stop --port $(DEV_PORT) 2>/dev/null || true
	@if ./bin/$(BINARY) --dir $(DEV_DIR) list 2>/dev/null | grep -q 'no exercises yet'; then \
		./bin/$(BINARY) --dir $(DEV_DIR) seed --force >/dev/null && echo "seeded demo data"; \
	fi
	./bin/$(BINARY) --dir $(DEV_DIR) serve --port $(DEV_PORT) >/dev/null 2>&1 & disown || true
	@sleep 1
	@echo "web UI: http://localhost:$(DEV_PORT)"

dev-stop:
	@-./bin/$(BINARY) --dir $(DEV_DIR) stop --port $(DEV_PORT) 2>/dev/null || echo "no dev server running"

dev-clean:
	rm -rf $(DEV_DIR)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin