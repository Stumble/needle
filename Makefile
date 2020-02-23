GO := go
NAME := needle
MAIN_GO := ./cmd/needle
ROOT_PACKAGE := $(GIT_PROVIDER)/$(ORG)/$(NAME)
GO_VERSION := $(shell $(GO) version | sed -e 's/^[^0-9.]*\([0-9.]*\).*/\1/')
PACKAGE_DIRS := $(shell $(GO) list ./... | grep -v /vendor/ | grep -v /api/ )
PKGS := $(shell go list ./... | grep -v /vendor | grep -v generated)
PKGS := $(subst  :,_,$(PKGS))
BUILDFLAGS := '-extldflags "-lm -lstdc++ -static"'
BUILDTAGS := netgo
CGO_ENABLED = 0
VENDOR_DIR=vendor

REDIS_DOCKER_NAME=$(NAME)-redis
REDIS_PORT=6379
MYSQL_DOCKER_NAME=$(NAME)-mysql
MYSQL_PASSWORD=my-secret
MYSQL_DB=$(NAME)_db
MYSQL_PORT=3306
MYSQL_TEST_DB=test_db

docker-mysql:
	docker run --name $(MYSQL_DOCKER_NAME) --rm -e MYSQL_ROOT_PASSWORD=$(MYSQL_PASSWORD) -e MYSQL_DATABASE=$(MYSQL_DB) -d -p $(MYSQL_PORT):3306 mysql:5.7
	sleep 15 # mysql needs some time to bootstrap.
	mysql -h localhost --user=root --password=$(MYSQL_PASSWORD) --protocol=tcp -e "DROP DATABASE IF EXISTS $(MYSQL_TEST_DB); CREATE DATABASE $(MYSQL_TEST_DB);"

docker-redis:
	docker run -d=true --name $(REDIS_DOCKER_NAME) --rm -p $(REDIS_PORT):6379 redis

docker-stop-mysql:
	docker stop $(MYSQL_DOCKER_NAME)

docker-stop-redis:
	docker stop $(REDIS_DOCKER_NAME)

test-run-deps: docker-mysql docker-redis
test-stop-deps: stop-docker-mysql stop-docker-redis

all: build

pre-build:
	git submodule update --init --recursive

generate:
	GO111MODULE=on go generate ./...

test-cmd:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -p 1 $(PACKAGE_DIRS) -test.v

test-local: pre-build
	make test-cmd

test: pre-build # add anything that's needed for test. ex. docker-mongo
	make test-cmd

build: pre-build generate
	GO111MODULE=on $(GO) build -o build/$(NAME) $(MAIN_GO)

install: pre-build generate
	cd cmd/needle && GO111MODULE=on $(GO) install

full: $(PKGS)

.PHONY: lint lint-fix
lint:
	@echo "--> Running linter"
	@golangci-lint run

lint-fix:
	@echo "--> Running linter auto fix"
	@golangci-lint run --fix
