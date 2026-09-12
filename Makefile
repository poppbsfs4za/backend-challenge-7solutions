.PHONY: run test test-race cover up down build lint

run:
	go run ./cmd/api

test:
	go test ./...

test-race:
	go test ./... -race

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

up:
	docker compose up --build -d

down:
	docker compose down

build:
	go build -o bin/api ./cmd/api