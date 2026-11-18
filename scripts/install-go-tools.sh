#!/usr/bin/env bash

set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
	echo "Go is required to install tooling." >&2
	exit 1
fi

export GOBIN="${GOBIN:-$(go env GOPATH)/bin}"
mkdir -p "$GOBIN"

go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
go install github.com/securego/gosec/v2/cmd/gosec@v2.29.0
go install github.com/fe3dback/go-arch-lint@v1.19.0
go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
go install github.com/goreleaser/goreleaser/v2@v2.15.1
