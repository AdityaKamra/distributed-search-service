.PHONY: up down restart logs build test clean

up:
	docker compose up -d --build

down:
	docker compose down -v

restart:
	docker compose restart

logs:
	docker compose logs -f

build:
	docker compose build --no-cache

test:
	chmod +x ./scripts/test_suite.sh
	./scripts/test_suite.sh

clean:
	docker compose down -v --remove-orphans
