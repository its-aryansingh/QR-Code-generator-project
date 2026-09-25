.PHONY: all up down logs migrate gen lint typecheck test check seed

all: check

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

migrate:
	cd services && go run cmd/migrate/main.go up

gen:
	cd services/db && sqlc generate
	pnpm --filter @qrit/api-client run gen

lint:
	cd services && golangci-lint run ./...
	pnpm run lint

typecheck:
	pnpm run typecheck

test:
	cd services && go test -v -race ./...
	pnpm run test

check: lint typecheck test

seed:
	cd services && go run cmd/api/main.go --seed
