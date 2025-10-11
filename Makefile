SHELL := /bin/bash

GOFILES := $(shell find . -name '*.go' -not -path './vendor/*')

GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
  GOBIN := $(shell go env GOPATH)/bin
endif

.PHONY: deps fmt lint lint-ci test run tui

GOFUMPT := $(GOBIN)/gofumpt
GOLANGCI := $(GOBIN)/golangci-lint

DEFAULT_GOTOOLS := \
	mvdan.cc/gofumpt@latest \
	github.com/golangci/golangci-lint/cmd/golangci-lint@latest

ENVFILE ?= .env

deps:
	@echo '==> Installing dev tools'
	@for tool in $(DEFAULT_GOTOOLS); do \
		echo "Installing $$tool"; \
		GO111MODULE=on GOBIN=$(GOBIN) go install $$tool; \
	done

fmt:
	@echo '==> Formatting Go sources'
	@go run mvdan.cc/gofumpt@latest -l -w $(GOFILES)
	@go run golang.org/x/tools/cmd/goimports@latest -w $(GOFILES)

lint:
	@echo '==> Running linters'
	@$(GOLANGCI) run ./...

lint-ci:
	@echo '==> Running linters (CI mode)'
	@$(GOLANGCI) run --out-format=github-actions ./...

TESTARGS ?=

test:
	@echo '==> Running tests'
	@go test ./... $(TESTARGS)

run:
	@echo '==> Starting kd-server API'
	@go run ./cmd/api

tui:
	@echo '==> Starting kd-server TUI'
	@go run ./cmd/tui
