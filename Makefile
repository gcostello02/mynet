.PHONY: build build-dashboard test fmt vet clean

build-dashboard:
	cd web/admin && npm ci && npm run build

build:
	go build -o bin/mynet ./cmd/mynet
	go build -o bin/mynet-server ./cmd/mynet-server

build-all: build-dashboard build

test:
	go test ./... -race -count=1

fmt:
	gofmt -w $$(go list -f '{{.Dir}}' ./...)

vet:
	go vet ./...

clean:
	rm -rf bin/
