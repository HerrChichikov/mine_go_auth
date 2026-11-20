.PHONY: help run build docker-build docker-run test clean docker-up docker-down docker-logs db-shell

APP_NAME := alpha
DOCKER_IMAGE := $(APP_NAME):latest

# Заставляем make использовать стандартную командную строку Windows, если мы на Windows
ifeq ($(OS),Windows_NT)
    # Команда удаления для Windows
    RM = rmdir /s /q
    # На Windows переменные окружения для Go задаются через env
    SET_ENV = set CGO_ENABLED=0&& set GOOS=linux&&
else
    RM = rm -rf
    SET_ENV = CGO_ENABLED=0 GOOS=linux
endif

# Упрощенная помощь, которая гарантированно работает везде без awk/grep
help:
	@echo Доступные команды:
	@echo run          - Запустить приложение
	@echo build        - Скомпилировать бинарник для Linux
	@echo docker-build - Собрать Docker образ
	@echo docker-run   - Запустить Docker контейнер
	@echo test         - Запустить тесты
	@echo clean        - Очистить бинарники и образы

run:
	go run ./cmd

build:
	$(SET_ENV) go build -o ./bin/$(APP_NAME) ./cmd

docker-build:
	docker build -f docker/Dockerfile -t $(DOCKER_IMAGE) .

docker-run:
	docker run --rm -p 3000:3000 $(DOCKER_IMAGE)

test:
	go test -v ./...

clean:
# Проверяем существование папки перед удалением в Windows
ifeq ($(OS),Windows_NT)
	if exist bin $(RM) bin
else
	$(RM) ./bin/$(APP_NAME)
endif
	docker rmi $(DOCKER_IMAGE) || true

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app

db-shell:
	docker compose exec db psql -U postgres -d microservice_db
