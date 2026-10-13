TYGO_VERSION := v0.2.21

.PHONY: gen web build

# Sinh web/src/api/types.gen.ts từ internal/server/api.
gen:
	go run github.com/gzuidhof/tygo@$(TYGO_VERSION) generate

# Build SPA vào web/dist (được go:embed).
web:
	pnpm -C web install --frozen-lockfile
	pnpm -C web build

build: web
	go build -o ainovel-cli ./cmd/ainovel-cli
