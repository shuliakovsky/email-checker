#!/bin/bash

# Запуск Docker Compose
docker compose up -d

# Ждем пока БД станет доступна
until pg_isready -h localhost -p 5432 -U postgres
do
  echo "Waiting for PostgreSQL..."
  sleep 2
done

# Применяем миграции (пример для migrate)
migrate -path ./migrations -database "postgres://postgres:strongpassword@localhost:5432/email_checker?sslmode=disable" up

# Собираем и запускаем приложение
go build -o email-checker ./cmd/email-checker
# PostgreSQL
POSTGRES_DB=email_checker
POSTGRES_USER=postgres
POSTGRES_PASSWORD=strongpassword
PG_HOST=localhost
PG_PORT=5432

# Redis
REDIS_NODES=localhost:6379
REDIS_DB=0

# Application
ADMIN_KEY=super-secret-admin-key-123

./email-checker \
  --server \
  --helo-domains="zbounce.net" \
  --redis=$REDIS_NODES \
  --pg-host=$PG_HOST \
  --pg-port=$PG_PORT \
  --pg-user=$POSTGRES_USER \
  --pg-password=$POSTGRES_PASSWORD \
  --pg-db=$POSTGRES_DB \
  --admin-key=$ADMIN_KEY
