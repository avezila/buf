
GO_SERVICES= buf
SERVICES = mongo $(GO_SERVICES)

DOCKER_FILES = \
	-p buf \
	-f ./docker-compose.yml \
	-f ./docker-compose-local.yml \
	`for S in $(GO_SERVICES); do echo -f ./$$S/build/docker-compose.yml; done;` \
	-f ./docker-compose.yml \
	-f ./docker-compose-local.yml

DOCKER_FILES_PROD = \
	-p buf \
	-f ./docker-compose.yml \
	-f ./docker-compose-prod.yml \
	-f ./postgres/docker-compose.yml \
	`for S in $(GO_SERVICES); do echo -f ./$$S/build/docker-compose.yml; done;` \
	-f ./docker-compose.yml \
	-f ./docker-compose-prod.yml


default: up
fresh: clean build up
up: up-quite logs
logs:
	cp .localhost.env .env
	@docker-compose $(DOCKER_FILES) logs -f --tail=1

up-quite:
	cp .localhost.env .env
	@docker-compose $(DOCKER_FILES) up --force-recreate -d $$s
reup:
	@./$$s/build/build.sh
	cp .localhost.env .env
	@docker-compose $(DOCKER_FILES) up -d --force-recreate $$s
	@docker-compose  $(DOCKER_FILES) logs -f

lite:
	@./buf/build/build.sh
	cat .localhost.env > .env
	# docker exec -it buf_postgres pgmigrate migrate -v -d /pgmigrate -c postgresql://postgres@localhost/buf -t 4000
	@docker-compose $(DOCKER_FILES) up -d --force-recreate buf
	@docker-compose  $(DOCKER_FILES) logs -f

ps:
	cp .localhost.env .env
	@docker-compose $(DOCKER_FILES) ps

config:
	cp .localhost.env .env
	@docker-compose $(DOCKER_FILES) config

down:
	cp .localhost.env .env
	@docker-compose $(DOCKER_FILES) down --remove-orphans $$s
clean:
	# @docker kill `docker ps -q` 2> /dev/null || echo -n
	# @docker rm `docker ps -aq`  2> /dev/null || echo -n
	# @docker volume rm `docker volume ls -q` 2> /dev/null || echo -n
	@docker-compose $(DOCKER_FILES) rm -f -v -s

build:
	echo -n "$(GO_SERVICES) " | xargs -P 16 -n 1 -d " " -i bash -c "./{}/build/build.sh"

prod-build: build
prod-fresh: prod-clean prod-build prod-up
prod-up: prod-up-quite prod-logs
prod-logs:
	cat .localhost.env > .env
	cat .prod.env >> .env
	@docker-compose $(DOCKER_FILES_PROD) logs -f --tail=1
prod-up-quite:
	cat .localhost.env > .env
	cat .prod.env >> .env

	@docker-compose $(DOCKER_FILES_PROD) up --force-recreate -d $$s
prod-reup:
	@./$$s/build/build.sh
	cat .localhost.env > .env
	cat .prod.env >> .env
	@docker-compose $(DOCKER_FILES_PROD) up -d --force-recreate $$s
	@docker-compose  $(DOCKER_FILES_PROD) logs -f

prod-lite:
	@./buf/build/build.sh
	cat .localhost.env > .env
	cat .prod.env >> .env
	# docker exec -it buf_postgres pgmigrate migrate -v -d /pgmigrate -c postgresql://postgres@localhost/buf -t 4000
	@docker-compose $(DOCKER_FILES_PROD) up -d --force-recreate buf
	@docker-compose  $(DOCKER_FILES_PROD) logs -f

prod-config:
	cat .localhost.env > .env
	cat .prod.env >> .env
	@docker-compose $(DOCKER_FILES_PROD) config

prod-down:
	cat .localhost.env > .env
	cat .prod.env >> .env
	@docker-compose $(DOCKER_FILES_PROD) down --remove-orphans $$s
prod-clean:
	@docker-compose $(DOCKER_FILES_PROD) rm -f -v -s

migrate:
	docker exec -it buf_postgres pgmigrate migrate -v -d /pgmigrate -c postgresql://postgres@localhost/buf -t 4000
