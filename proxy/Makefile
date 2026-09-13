.PHONY: run build test bench

run:
	go run ./cmd/proxy

build:
	go build -o bin/proxy ./cmd/proxy

test:
	go test ./...

# Runs baseline (normal) + shaping k6 suites with automatic /admin/mode flips.
# Requires: proxy on :8080, Redis, ADMIN_TOKEN in the environment.
bench:
	@bash ../scripts/loadtest/run-bench.sh
