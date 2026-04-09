APP_NAME := app
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP_NAME)
IMAGE ?= usdt-rates:latest

GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.24.1
MIGRATIONS_DIR := migrations
POSTGRES_HOST ?= 127.0.0.1
POSTGRES_PORT ?= 5432

GOLANGCI_LINT := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0

PROTO_DIR := proto
GEN_DIR   := gen

.PHONY: build test docker-build run lint proto install-goose migrate-up migrate-down

build:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(BIN) ./cmd/api

test:
	go test ./... -count=1

docker-build:
	docker compose build

run:
	go run ./cmd/api

lint:
	go run $(GOLANGCI_LINT) run ./...

proto:
	find $(PROTO_DIR) -name '*.proto' -exec \
	  protoc \
	    --proto_path=$(PROTO_DIR) \
	    --proto_path=$(shell brew --prefix protobuf 2>/dev/null || echo /usr)/include \
	    --go_out=$(GEN_DIR) --go_opt=module=usdt-rates/$(GEN_DIR) \
	    --go-grpc_out=$(GEN_DIR) --go-grpc_opt=module=usdt-rates/$(GEN_DIR) \
	  {} \;

install-goose:
	go install github.com/pressly/goose/v3/cmd/goose@v3.24.1

migrate-up:
	@test -f postgres.env || (echo "postgres.env missing (copy postgres.env.example)" >&2; exit 1)
	set -a && . ./postgres.env && set +a && \
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@$(POSTGRES_HOST):$(POSTGRES_PORT)/$$POSTGRES_DB?sslmode=disable" up

migrate-down:
	@test -f postgres.env || (echo "postgres.env missing (copy postgres.env.example)" >&2; exit 1)
	set -a && . ./postgres.env && set +a && \
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@$(POSTGRES_HOST):$(POSTGRES_PORT)/$$POSTGRES_DB?sslmode=disable" down
