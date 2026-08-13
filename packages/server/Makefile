.PHONY: run build clean test lint

run:
	go run cmd/server/main.go

build:
	go build -o bin/server cmd/server/main.go

clean:
	rm -rf bin/

test:
	go test ./...

lint:
	golangci-lint run
