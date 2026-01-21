.PHONY: all build run test clean proto sqlc docker-up docker-down

# Variáveis
DB_URL=postgresql://root:root@localhost:5432/gofleet?sslmode=disable

# --- Code Generation ---

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
	--go-grpc_out=. --go-grpc_opt=paths=source_relative \
	internal/infra/grpc/protofiles/*.proto

sqlc:
	docker run --rm -v $(PWD):/src -w /src kjconroy/sqlc generate

new-migration:
	migrate create -ext=sql -dir=sql/migrations -seq $(name)

# --- Infraestrutura & Docker ---

docker-up:
	docker-compose up -d --build

docker-down:
	docker-compose down -v

migrate-up:
	migrate -path=sql/migrations -database "$(DB_URL)" -verbose up

migrate-down:
	migrate -path=sql/migrations -database "$(DB_URL)" -verbose down

# --- Execução Local (Sem Docker para os Apps) ---
run-api:
	go run cmd/api/main.go

run-worker:
	go run cmd/worker/main.go

run-fleet:
	go run cmd/fleet/main.go

# --- Qualidade ---
test:
	go test ./... -v

tidy:
	go mod tidy