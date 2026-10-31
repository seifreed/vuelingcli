.PHONY: check fmt vet tidy test build lint vuln cover cover-check

GOVULNCHECK_VERSION ?= v1.9.0
GOLANGCI_VERSION ?= v2.15.0
COVER_MIN ?= 63

check: fmt vet tidy test build

fmt:
	test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*'))"

vet:
	go vet ./...

tidy:
	go mod tidy -diff

test:
	go test -race ./...

cover:
	go test -coverprofile=cover.out ./...
	@go tool cover -func=cover.out | tail -1

cover-check: cover
	@total=$$(go tool cover -func=cover.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
	echo "total coverage: $$total% (minimum $(COVER_MIN)%)"; \
	awk -v t="$$total" -v m="$(COVER_MIN)" 'BEGIN { exit (t+0 >= m+0) ? 0 : 1 }' \
		|| { echo "coverage below the floor"; exit 1; }

build:
	@tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; go build -o "$$tmp/vueling" ./cmd/vueling

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
