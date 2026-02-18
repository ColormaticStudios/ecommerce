#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC_FILE="${ROOT_DIR}/api/openapi.yaml"
GO_CONFIG_FILE="${ROOT_DIR}/api/oapi-codegen.yaml"
TS_OUTPUT_FILE="${ROOT_DIR}/frontend/src/lib/api/generated/openapi.ts"

if ! command -v go >/dev/null 2>&1; then
	echo "go is required to generate backend contract types"
	exit 1
fi

if ! command -v bunx >/dev/null 2>&1; then
	echo "bunx is required to generate frontend contract types"
	exit 1
fi

mkdir -p "${ROOT_DIR}/internal/apicontract"
mkdir -p "$(dirname -- "${TS_OUTPUT_FILE}")"

go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 \
	-config "${GO_CONFIG_FILE}" \
	"${SPEC_FILE}"

(
	cd "${ROOT_DIR}/frontend"
	bunx openapi-typescript ../api/openapi.yaml -o src/lib/api/generated/openapi.ts
)

echo "Generated contract files:"
echo " - internal/apicontract/openapi.gen.go"
echo " - frontend/src/lib/api/generated/openapi.ts"
