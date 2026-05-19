#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
FILES=(
	"internal/apicontract/openapi.gen.go"
	"frontend/src/lib/api/generated/openapi.ts"
)

"${ROOT_DIR}/scripts/generate-api-contracts.sh"

if [ -n "$(git -C "${ROOT_DIR}" status --porcelain -- "${FILES[@]}")" ]; then
	echo "Generated API contract files are out of date."
	git -C "${ROOT_DIR}" --no-pager diff -- "${FILES[@]}"
	exit 1
fi

echo "Generated API contract files match HEAD."
