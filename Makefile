project_name = alerts-ingester

.DEFAULT_GOAL := help

.PHONY: help
help: ## Print this help message
	@echo "List of available make commands";
	@echo "";
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}';

.PHONY: dev
dev: ## Run docker compose and all services
	docker compose --env-file local.env up --build -d

.PHONY: demo
demo: ## starts up every service including the ingester. Will not detach, will build all compilable services from scratch
	ENV=demo docker compose --env-file demo.env --profile demo up --build

.PHONY: down
down: ## Stop all docker compose services
	docker compose down

.PHONY: tools
tools: ## show a list of tools used for this project
	@echo "List of tools used\n"
	@echo "golang v1.25 \n\t https://go.dev/dl/"
	@echo "docker and docker compose \n\t https://docs.docker.com/compose/"
	@echo "sqlc \n\t https://sqlc.dev/"
	@echo "golang-migrate \n\t https://github.com/golang-migrate/migrate"

.PHONY: test
test: ## test the project
	@echo running tests
	go test ./...

build: ## build the project
	@echo running build creation
	go build cmd/alerts-ingester/main.go

.PHONY: cover
cover: ## Run tests with coverage file provided to coverage.out
	@echo running coverage tests
	go test -coverprofile=coverage.out ./...

.PHONY: cover-html
cover-html: cover ## Run tests with coverage file and html output for viewing
	@echo Generating html cover report
	go tool cover -html=coverage.out

gen-queries: ## generate sqlc queries
	@echo generating queries and models via sqlc
	sqlc generate

.PHONY: migrate-up
migrate-up: ## migrate the sqlite database (shoved into db for our purposes)
	@echo running migrations up
	migrate -database "sqlite3://db/alerts.db" -path ./db/migrations/ up

.PHONY: migrate-down
migrate-down: ## migreate down the sqlite database (shoved into db for our purposes)
	@echo running migrations down
	migrate -database "sqlite3://db/alerts.db" -path ./db/migrations/ down
