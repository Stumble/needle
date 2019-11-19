GO := go
NAME := dcache
REDIS_DOCKER_NAME=$(NAME)-redis
REDIS_PORT=6379
CGO_ENABLED = 0
GO111MODULE = on

docker-redis:
	docker run -d=true --name $(REDIS_DOCKER_NAME) -p $(REDIS_PORT):6379 redis

stop-docker-redis:
	docker stop $(REDIS_DOCKER_NAME)
	docker rm $(REDIS_DOCKER_NAME)

test-start-all:
	make docker-redis

test-stop-all:
	make stop-docker-redis

test-cmd:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -p 1 . -test.v

test: test-start-all
	GO111MODULE=$(GO111MODULE) make test-cmd && make test-stop-all || (make test-stop-all; exit 2)
