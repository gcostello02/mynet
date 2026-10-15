.PHONY: build test fmt vet clean

build:
	go build -o bin/mynet ./cmd/mynet
	go build -o bin/mynet-server ./cmd/mynet-server

test:
	go test ./... -race -count=1

fmt:
	gofmt -w $$(go list -f '{{.Dir}}' ./...)

vet:
	go vet ./...

clean:
	rm -rf bin/
