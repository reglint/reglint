


.PHONY: help quality all format lint test coverage test-coverage test-race test-flaky test-e2e-smoke test-e2e mutation test-mutation \
	security arch build run analyze-example analyze-fail release-check release-snapshot

GLOBAL_TARGETS ?=
BUILD_OUT ?= bin/reglint
ARGS ?=
FLAKY_COUNT ?=20

help:
	@printf "%s\n" \
	"Common targets:" \
	"  make quality         Run full quality checks" \
	"  make format          Run gofmt on tracked Go files" \
	"  make lint            Run golangci-lint" \
	"  make test            Run tests with coverage gate" \
	"  make test-race       Run tests with race detector" \
	"  make test-flaky      Run tests repeatedly to detect flakes" \
	"  make test-e2e-smoke  Run compiled-binary e2e smoke scenarios" \
	"  make test-e2e        Run compiled-binary full e2e matrix" \
	"  make coverage        Run coverage gate only" \
	"  make mutation        Run mutation testing (final stage)" \
	"  make security        Run govulncheck and gosec" \
	"  make arch            Run go-arch-lint" \
	"  make build           Build CLI binary to $(BUILD_OUT)" \
	"  make run ARGS='...'  Run CLI from source" \
	"  make analyze-example Analyze test fixtures with example config" \
	"  make analyze-fail    Analyze test fixtures with failOn config" \
	"  make release-check    Validate goreleaser config" \
	"  make release-snapshot Build release artifacts locally (no publish)"

quality: test lint test-race test-flaky test-coverage test-mutation security arch

format:
	gofmt -w $$(git ls-files '*.go')

lint:
	golangci-lint run --timeout 5m

test:
	go test ./...

coverage: test-coverage

test-coverage:
	@coverprofile="$$(mktemp -t quality-cover.XXXXXX)"; \
	go test -count=1 -coverprofile="$$coverprofile" -covermode=atomic -coverpkg=./... ./...; \
	total="$$(go tool cover -func="$$coverprofile" | awk '/^total:/{gsub(/%/,"",$$3); print $$3}')"; \
	rm -f "$$coverprofile"; \
	if ! awk -v total="$$total" -v minimum="90" 'BEGIN {exit !(total >= minimum)}'; then \
		echo "Coverage $${total}% is below required 90%." >&2; \
		exit 1; \
	fi

test-race:
	go test -race ./...

test-flaky:
	go test -count=$(FLAKY_COUNT) -shuffle=on ./...

test-e2e-smoke:
	go test -count=1 ./cmd/reglint -run '^TestE2ESmoke'

test-e2e:
	go test -count=1 ./cmd/reglint -run '^TestE2E(Smoke|Full)'

test-mutation:
	gremlins unleash $(ARGS)

mutation: test-mutation

security:
	govulncheck ./...
	gosec ./...

arch:
	go-arch-lint check

release-check:
	goreleaser check

release-snapshot:
	goreleaser release --snapshot --clean

build:
	go build -o $(BUILD_OUT) ./cmd/reglint

run:
	go run ./cmd/reglint $(ARGS)

analyze-example:
	go run ./cmd/reglint analyze --config testdata/rules/example.yaml testdata/fixtures

analyze-fail:
	go run ./cmd/reglint analyze --config testdata/rules/fail.yaml testdata/fixtures
