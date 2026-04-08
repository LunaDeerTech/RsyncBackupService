.PHONY: build dev dev-backend dev-frontend clean test docker

build:
	cd web && npm ci && npm run build
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/rbs ./cmd/server/main.go

dev:
	@echo "frontend: make dev-frontend"
	@echo "backend: make dev-backend"

dev-backend:
	RBS_DEV_MODE=true go run ./cmd/server/main.go

dev-frontend:
	cd web && npm run dev

test:
	go test ./...

docker:
	docker build -t rbs:latest .

clean:
	rm -rf bin web/dist
	mkdir -p web/dist
	touch web/dist/.gitkeep