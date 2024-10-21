# Variables
main_path = main.go
binary = sclng-backend-test

.DEFAULT_GOAL:=help
help: ## Show this help.
	@echo ''
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    %-20s%s\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  %s\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)

all: audit build

build: ## build the go application
	make clean
	make tidy
	mkdir -p bin/
	go build $(LDFLAGS) -o bin/${binary} ${main_path}

tidy: ## download dependencies
	go mod tidy
	go mod download

test: ## runs tests and create generates coverage report
	make tidy
	make vendor
	mkdir -p tmp/
	go test -v -timeout 10m ./... -coverprofile=tmp/coverage.out -json > tmp/report.json

audit: ## runs code quality checks
	go mod verify
	go fmt ./...
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

clean: ## cleans binary and other generated files
	go clean
	rm -f tmp/coverage.out tmp/report.json bin/${binary}

lint: ## go linting
	make tidy
	golangci-lint run

coverage: ## displays test coverage report in html mode
	make test
	go tool cover -html=tmp/coverage.out

.PHONY: vendor
vendor: ## all packages required to support builds and tests in the /vendor directory
	go mod vendor