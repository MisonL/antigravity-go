.PHONY: build frontend backend clean run update-core

# Default target
all: build

# Build everything
build: frontend backend

# Build frontend and copy dist for embedding
frontend:
	cd frontend && bun install && bun run build
	rm -rf internal/server/dist
	cp -r frontend/dist internal/server/dist

# Build backend
backend:
	go build -o ago ./cmd/ago

# Clean build artifacts
clean:
	rm -f ago
	rm -rf internal/server/dist
	rm -rf frontend/dist

# Update core binary from system
update-core:
	bash ./scripts/update_core.sh

# Run locally in web mode
run: build
	./ago --web --no-tui
