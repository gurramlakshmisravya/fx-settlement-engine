.PHONY: build run test docker-up docker-down proto help

# Default target
all: test build

## build: Builds the server binary
build:
	@echo "Building FX Settlement Engine..."
	go build -o bin/settlement-engine ./cmd/server

## run: Runs the application locally
run:
	@echo "Running FX Settlement Engine locally..."
	go run ./cmd/server/main.go

## test: Runs unit and integration tests
test:
	@echo "Running tests..."
	go test -v ./...

## docker-up: Starts all services with Docker Compose
docker-up:
	@echo "Starting PostgreSQL, Redis, Kafka, and FX Settlement Engine..."
	docker-compose up --build -d

## docker-down: Stops and cleans up Docker Compose resources
docker-down:
	@echo "Stopping Docker containers..."
	docker-compose down -v

## proto: Compiles protobuf files if protoc is installed
proto:
	@echo "Generating Protobuf & gRPC code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/settlement.proto

## help: Displays help for Makefile commands
help:
	@echo "Usage:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
