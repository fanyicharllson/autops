.PHONY: run build test bench

run:
	go run ./cmd/proxy

build:
	go build -o bin/proxy ./cmd/proxy

test:
	go test ./...

# TODO: wire up a real k6 load test once scripts/loadtest is finalized.
bench:
	k6 run ../scripts/loadtest/spike.js
