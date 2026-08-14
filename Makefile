.PHONY: generate tidy build run docker-up docker-down docker-scale

generate:
	buf generate

tidy:
	go mod tidy

build: generate tidy
	go build -o bin/server ./cmd/server

run: build
	./bin/server

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-scale:
	docker compose up --build -d --scale chat=2
