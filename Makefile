GO := go
NAME := needle
MAIN_GO := ./cmd/needle
GO_VERSION := $(shell $(GO) version | sed -e 's/^[^0-9.]*\([0-9.]*\).*/\1/')
CGO_ENABLED = 0

REDIS_DOCKER_NAME=$(NAME)-redis
REDIS_PORT=6379
MYSQL_DOCKER_NAME=$(NAME)-mysql
MYSQL_PASSWORD=my-secret
MYSQL_DB=$(NAME)_db
MYSQL_PORT=3306
MYSQL_TEST_DB=test_db

.PHONY: docker-mysql-start docker-mysql-stop
docker-mysql-start:
	docker run --name $(MYSQL_DOCKER_NAME) --rm -e MYSQL_ROOT_PASSWORD=$(MYSQL_PASSWORD) -e MYSQL_DATABASE=$(MYSQL_DB) -d -p $(MYSQL_PORT):3306 mysql:5.7
	sleep 10 # mysql needs some time to bootstrap.
docker-mysql-stop:
	docker stop $(MYSQL_DOCKER_NAME)

.PHONY: docker-redis-start docker-redis-stop
docker-redis-start:
	docker run -d --rm --name $(REDIS_DOCKER_NAME) -p $(REDIS_PORT):6379 redis
docker-redis-stop:
	docker stop $(REDIS_DOCKER_NAME)

.PHONY: test-start-all-deps test-stop-all-deps
test-start-all-deps: docker-mysql-start docker-redis-start
test-stop-all-deps: docker-mysql-stop docker-redis-stop

.PHONY: test-cmd test-local test
test-cmd:
	export ENV=test && \
	go test pkg/... -test.v -p 1

test-local: test-start-all-deps
	make test-cmd && (make test-stop-all-deps) || (make test-stop-all-deps; exit 2)

test:
	make test-cmd

pre-build:
	git submodule update --init --recursive

build: pre-build
	GO111MODULE=on $(GO) build -o build/$(NAME) $(MAIN_GO)

install: pre-build
	cd cmd/needle && GO111MODULE=on $(GO) install

.PHONY: lint lint-fix
lint:
	@echo "--> Running linter"
	@golangci-lint run

lint-fix:
	@echo "--> Running linter auto fix"
	@golangci-lint run --fix
