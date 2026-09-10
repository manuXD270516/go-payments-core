.PHONY: up down migrate run test lint fmt tidy

up:        ## Levanta la infraestructura local
	docker compose up -d

down:      ## Baja la infraestructura
	docker compose down

migrate:   ## Aplica migraciones versionadas
	go run ./cmd/migrate up

run:       ## Corre la API
	go run ./cmd/api

test:      ## Tests con detector de carreras
	go test ./... -race -count=1

cover:     ## Cobertura del dominio
	go test ./internal/domain/... -coverprofile=coverage.out && go tool cover -func=coverage.out

lint:      ## Análisis estático
	go vet ./... && golangci-lint run

fmt:       ## Formato
	gofmt -s -w .

tidy:
	go mod tidy
