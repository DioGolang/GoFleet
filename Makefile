createmigration:
	migrate create -ext=sql -dir=sql/migrations -seq init

migrateup:
	migrate -path=sql/migrations -database "postgresql://root:root@localhost:5432/gofleet?sslmode=disable" -verbose up

migratedown:
	migrate -path=sql/migrations -database "postgresql://root:root@localhost:5432/gofleet?sslmode=disable" -verbose down

sqlc:
	docker run --rm -v $(PWD):/src -w /src kjconroy/sqlc generate

run:
	go run cmd/api/main.go

.PHONY: createmigration migrateup migratedown sqlc run