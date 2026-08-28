.PHONY: build run test lint docker-up docker-down migrate fmt vet clean

APP_NAME := soundstage
MAIN := ./cmd/server

build:
	go build -o bin/$(APP_NAME) $(MAIN)

run:
	go run $(MAIN) -config configs/config.yaml

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

docker-up:
	docker compose -f deployments/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker-compose.yml down

clean:
	rm -rf bin/ tmp/
