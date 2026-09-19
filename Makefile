BINARY  := guitar-coach
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/luca-filipponi/guitar-coach/internal/cli.version=$(VERSION)
DEV_DIR := .dev-data
DEV_PORT ?= 8080

# test-session runs a short, silent practice session against a throwaway data
# dir (TEST_DIR), so nothing is recorded into your real ~/.guitar-coach store.
# It is meant for exercising the interactive flow (prompts, countdowns, rest)
# on a real terminal. End the last prompt with "n" to finish; the temp dir and
# its sessions are discarded with `make test-session-clean`.
TEST_DIR := .test-data

.PHONY: test-session test-session-clean

test-session: build
	@set -e; \
	if [ ! -f "$(TEST_DIR)/guitar-coach.db" ]; then \
		GUITAR_COACH_DIR=$(TEST_DIR) ./bin/$(BINARY) seed >/dev/null && \
		echo "seeded test exercises into $(TEST_DIR)"; \
	fi
	@GUITAR_COACH_DIR=$(TEST_DIR) ./bin/$(BINARY) start \
		--exercises 3 --duration 1m --rest 30s --break 1m \
		--warmup 2m --alarm-volume 0 --alarm-count 1

test-session-clean:
	rm -rf $(TEST_DIR)

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