.PHONY: build build-backend build-frontend test test-backend test-frontend run

build: build-frontend build-backend

build-backend:
	mkdir -p bin
	go build -o bin/rsync-backup-service ./cmd/server

build-frontend:
	npm --prefix web run build

test: test-backend test-frontend

test-backend:
	go test ./...

test-frontend:
	npm --prefix web run test

run:
	go run ./cmd/server