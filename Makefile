BINARY := guitar-coach

.PHONY: build run test vet fmt clean

build:
	go build -o bin/$(BINARY) .

run:
	go run .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin