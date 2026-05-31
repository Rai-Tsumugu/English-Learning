.PHONY: help dev run build test fmt vet tidy migrate ingest web-dev web-build clean backup

BIN := bin/app
PKG := ./...

help:
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-14s %s\n", $$1, $$2}'

dev: ## Run API + Vite in parallel
	@$(MAKE) -j2 run web-dev

run: ## Run Go API server
	go run ./cmd/app serve

build: ## Build Go binary with embedded web assets
	cd web && npm run build
	go build -o $(BIN) ./cmd/app

test: ## Run Go tests
	go test -race -count=1 $(PKG)

fmt:
	go fmt $(PKG)

vet:
	go vet $(PKG)

tidy:
	go mod tidy

migrate: ## Run DB migrations
	go run ./cmd/app migrate up

ingest: ## Ingest word data (NGSL/CEFR-J/Octanove)
	go run ./cmd/app ingest

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

clean:
	rm -rf bin/ web/dist/

backup: ## Encrypted backup of data/*.db with age (AGE_RECIPIENT=age1...)
	./scripts/backup.sh $(AGE_RECIPIENT)
