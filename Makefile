COMPOSE   ?= docker compose
GOOSE     ?= go run github.com/pressly/goose/v3/cmd/goose@v3.24.3
CONNECTORS := services/connectors

ifneq (,$(wildcard .env))
include .env
endif

export ADZUNA_APP_ID ADZUNA_APP_KEY ADZUNA_COUNTRY ADZUNA_WHAT ADZUNA_WHERE
export POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB POSTGRES_DSN POSTGRES_PORT

POSTGRES_DSN ?= postgres://jobsonar:jobsonar@localhost:5432/jobsonar?sslmode=disable

.PHONY: up down migrate test connector demo wait-db

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

wait-db:
	@echo "waiting for postgres..."
	@i=0; \
	until $(COMPOSE) exec -T postgres pg_isready -U $${POSTGRES_USER:-jobsonar} -d $${POSTGRES_DB:-jobsonar} >/dev/null 2>&1; do \
		i=$$((i+1)); \
		if [ $$i -ge 30 ]; then echo "postgres did not become ready"; exit 1; fi; \
		sleep 1; \
	done

migrate: wait-db
	$(GOOSE) -dir db/migrations postgres "$(POSTGRES_DSN)" up

test:
	cd $(CONNECTORS) && go test ./...

connector:
	cd $(CONNECTORS) && go run ./cmd/connector

# Week 1 demo: local stack + schema + Adzuna JSON lines on stdout.
# Needs ADZUNA_APP_ID / ADZUNA_APP_KEY in the environment or .env.
demo: up migrate
	$(MAKE) connector
