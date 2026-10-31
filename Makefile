.PHONY: check fmt vet test build vuln

GOVULNCHECK_VERSION ?= v1.9.0

check: fmt vet test build

fmt:
	test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*'))"

vet:
	go vet ./...

test:
	go test -race ./...

build:
	@tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; go build -o "$$tmp/vueling" ./cmd/vueling

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
