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

# 覆盖率门禁：单命令合并全包覆盖率，低于门槛则非零退出
# 用法: make coverage-check [COV_THRESHOLD=80]
COV_THRESHOLD ?= 80
coverage:
	go test ./... -count=1 -covermode=set -coverpkg=./... -coverprofile=coverage.out

coverage-check:
	go test ./... -count=1 -covermode=set -coverpkg=./... -coverprofile=coverage.out
	@total=$$(go tool cover -func=coverage.out | grep -oE 'total:.*[0-9]+\.[0-9]+%' | grep -oE '[0-9]+\.[0-9]+' | head -1); \
	int=$${total%%.*}; \
	echo "total coverage: $$total%% (threshold: $(COV_THRESHOLD)%%)"; \
	if [ -z "$$int" ] || [ "$$int" -lt "$(COV_THRESHOLD)" ]; then \
		echo "ERROR: 覆盖率 '$$total'%% 低于门禁 $(COV_THRESHOLD)%%"; rm -f coverage.out; exit 1; \
	else echo "PASS: 覆盖率达标"; fi; \
	rm -f coverage.out
