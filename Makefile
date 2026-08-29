.PHONY: build run test lint docker-up docker-down migrate fmt vet clean proto

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

proto:
	@which protoc > /dev/null || (echo "protoc not installed" && exit 1)
	@which protoc-gen-go > /dev/null || go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@which protoc-gen-go-grpc > /dev/null || go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/*.proto

clean:
	rm -rf bin/ tmp/
