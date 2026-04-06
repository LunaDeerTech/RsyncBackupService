.PHONY: build dev clean

build:
	cd web && npm ci && npm run build
	mkdir -p bin
	go build -o bin/rbs ./cmd/server

dev:
	@echo "frontend: cd web && npm run dev"
	@echo "backend: RBS_DEV_MODE=true go run ./cmd/server"

clean:
	rm -rf bin web/dist
	mkdir -p web/dist
	touch web/dist/.gitkeep