SOURCE_FILES?=./...
TEST_PATTERN?=.
TEST_OPTIONS?=
BUILD_NAME?=alertmanager-config-controller

export PATH := ./bin:$(PATH)
export GO111MODULE := on

# Install all the build and lint dependencies
setup:
	go mod download
	# curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s v1.16.0
.PHONY: setup

# Run all the tests
test:
	go test $(TEST_OPTIONS) -failfast -race -coverpkg=./... -covermode=atomic -coverprofile=coverage.txt $(SOURCE_FILES) -run $(TEST_PATTERN) -timeout=2m
.PHONY: test

# Run all the tests and opens the coverage report
cover: test
	go tool cover -html=coverage.txt
.PHONY: cover

# gofmt and goimports all go files
fmt:
	find . -name '*.go' -not -wholename './vendor/*' | while read -r file; do gofmt -w -s "$$file"; goimports -w "$$file"; done
.PHONY: fmt

# Run all the linters
lint:
	./bin/golangci-lint run --tests=false --enable-all --disable=gochecknoglobals,dupl,interfacer ./...
.PHONY: lint

# Run all the tests and code checks
ci: build-ci test lint
.PHONY: ci

# Build the controller in ci for alpine image
# Note: output will be the dir root to make it work with travis deploy
build-ci:
	GOOS=linux GOARCH=amd64 go build -v -i -o ./$(BUILD_NAME) ./cmd
.PHONY: build-ci

# Build the controller
build:
	go build -v -i -o ./bin/$(BUILD_NAME) ./cmd
.PHONY: build

# Show to-do items per file.
todo:
	@grep \
		--exclude-dir=vendor \
		--exclude=Makefile \
		--text \
		--color \
		-nRo -E ' TODO:.*|SkipNow' .
.PHONY: todo

.DEFAULT_GOAL := build

