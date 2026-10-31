.PHONY: check fmt vet tidy test build lint vuln

GOVULNCHECK_VERSION ?= v1.9.0
GOLANGCI_VERSION ?= v2.15.0

check: fmt vet tidy test build

fmt:
	test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*'))"

vet:
	go vet ./...

tidy:
	go mod tidy -diff

test:
	go test -race ./...

build:
	@tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; go build -o "$$tmp/vueling" ./cmd/vueling

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
