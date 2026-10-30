.PHONY: check fmt vet test build

check: fmt vet test build

fmt:
	test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*'))"

vet:
	go vet ./...

test:
	go test -race ./...

build:
	go build ./cmd/vueling
