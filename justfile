BUILD_FLAGS := env("BUILD_FLAGS", "")
TEST_FLAGS := env("TEST_FLAGS", "")
GOLANGCI_LINT_VERSION := env("GOLANGCI_LINT_VERSION", "latest")

all: fmt tidy build lint test

build:
	go build {{BUILD_FLAGS}} ./...

builddir:
	mkdir -p -m 0777 build

lint:
	golangci-lint run

clean:
	rm -rf build/*

fmt:
	go fmt ./...

test: (tests "")

tests *flags:
	go test -race {{BUILD_FLAGS}} {{TEST_FLAGS}} {{ flags }} ./...

# creates a test coverage report, and produces json test output.  useful for ci.
cover: builddir
	go test {{TEST_FLAGS}} -v -covermode=count -coverprofile=build/coverage.out -json ./...
	go tool cover -html=build/coverage.out -o build/coverage.html

tidy:
	go mod tidy

update:
	go get -u ./...
	go mod tidy

### TOOLS

# installs tools used during build
tools:
	sh -c "$(wget -O - -q https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh || echo exit 2)" -- -b `go env GOPATH`/bin {{GOLANGCI_LINT_VERSION}}


