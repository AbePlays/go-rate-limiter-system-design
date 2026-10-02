.PHONY: compose-up compose-down fmt vet test build

compose-up:
	docker compose up -d

compose-down:
	docker compose down

fmt:
	test -z "$$(gofmt -l .)"

vet:
	go vet ./...

test:
	go test -p 1 ./... -race -count=1

build:
	go build ./...
