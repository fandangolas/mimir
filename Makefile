.PHONY: build run test lint up down

build:
	go build -o bin/assistant ./cmd/assistant

run: build
	./bin/assistant

test:
	go test ./...

lint:
	golangci-lint run ./...

up:
	docker compose up -d

down:
	docker compose down
