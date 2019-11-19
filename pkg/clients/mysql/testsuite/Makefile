NAME := MetaTest
MYSQL_DOCKER_NAME=$(NAME)-mysql
MYSQL_PASSWORD=my-secret
MYSQL_PORT=3306

docker-mysql-start:
	docker run --name $(MYSQL_DOCKER_NAME) --rm -e MYSQL_ROOT_PASSWORD=$(MYSQL_PASSWORD) -d -p $(MYSQL_PORT):3306 mysql:5.7
	sleep 10 # mysql needs some time to bootstrap.

docker-mysql-stop:
	docker stop $(MYSQL_DOCKER_NAME)
