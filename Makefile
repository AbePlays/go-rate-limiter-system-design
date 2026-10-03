.PHONY: compose-up compose-down fmt vet vuln test build

compose-up:
	docker compose up -d

compose-down:
	docker compose down

fmt:
	test -z "$$(gofmt -l .)"

vet:
	go vet ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

test:
	go test -p 1 ./... -race -count=1

build:
	go build ./...
