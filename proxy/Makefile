.PHONY: run build test bench clean-bench

BENCH_DIR := ../bench-results

run:
	go run ./cmd/proxy

build:
	go build -o bin/proxy ./cmd/proxy

test:
	go test ./...

# Runs baseline (normal) + shaping k6 suites with automatic /admin/mode flips.
# Requires: proxy on :8080, Redis, ADMIN_TOKEN in the environment.
# Keeps the 5 most recent baseline-*.json and shaping-*.json under bench-results/.
bench:
	@mkdir -p $(BENCH_DIR)
	@for kind in baseline shaping; do \
		ls -1t $(BENCH_DIR)/$${kind}-*.json 2>/dev/null | tail -n +6 | while IFS= read -r f; do \
			rm -f "$$f"; \
		done; \
	done
	@bash ../scripts/loadtest/run-bench.sh

# Wipe all saved k6 summary exports.
clean-bench:
	@mkdir -p $(BENCH_DIR)
	@rm -f $(BENCH_DIR)/baseline-*.json $(BENCH_DIR)/shaping-*.json
	@echo "cleared $(BENCH_DIR)"
