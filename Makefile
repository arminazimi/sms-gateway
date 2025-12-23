build:
	go build ./...

run:
	go run ./cmd/api

loadtest:
	go run ./cmd/loadtest -base-url http://localhost:8080 -rps 1000 -duration 30s -concurrency 200 -users 5000 -recipients 1 -express-ratio 0.2

seed:
	DB_HOST=localhost DB_PORT=3306 DB_USER_NAME=sms_user DB_PASSWORD=sms_pass DB_NAME=sms_gateway \
	go run ./cmd/loadtest -seed-only -seed-method db -seed-balance 100000 -seed-timeout 5m -users 5000

lint:
	golangci-lint run

test:
	go test ./...

tidy:
	go mod tidy

swag:
	swag fmt && swag init -g cmd/api/main.go -o docs

docker:
	docker compose up --build

