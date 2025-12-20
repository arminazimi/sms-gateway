build:
	go build ./...

run:
	go run ./cmd/api

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

