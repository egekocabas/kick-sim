.PHONY: generate web test build

generate:
	go run ./cmd/openapi-gen --output ./web/openapi.json
	cd web && npm run api:generate

web: generate
	cd web && npm run build

test: generate
	test -z "$$(gofmt -l .)"
	go test -race ./...
	go vet ./...
	cd web && npm run typecheck && npm test && npm run build
	git diff --exit-code -- web/openapi.json web/src/api/generated
	go test -tags studio_embed ./web ./internal/studio

build: web
	go build -trimpath -tags studio_embed -o ./dist/kick-sim ./cmd/kick-sim
