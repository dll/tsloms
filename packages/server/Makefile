.PHONY: run build clean test lint coverage coverage-check

run:
	go run cmd/server/main.go

build:
	go build -o bin/server cmd/server/main.go

clean:
	rm -rf bin/
	rm -f coverage.out

test:
	go test ./...

lint:
	golangci-lint run

# 覆盖率门禁：合并全包覆盖率，跨平台浮点比较
# 用法: make coverage-check [COV_THRESHOLD=80]
COV_THRESHOLD ?= 80
coverage:
	go test ./... -count=1 -covermode=set -coverpkg=./... -coverprofile=coverage.out

coverage-check:
	go test ./... -count=1 -covermode=set -coverpkg=./... -coverprofile=coverage.out
	go run ./cmd/coveragecheck -threshold $(COV_THRESHOLD) -profile coverage.out
	rm -f coverage.out