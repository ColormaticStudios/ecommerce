#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

FILES=(
	"internal/apicontract/openapi.gen.go"
	"frontend/src/lib/api/generated/openapi.ts"
)

for file in "${FILES[@]}"; do
	source="${ROOT_DIR}/${file}"
	target="${TMP_DIR}/${file}"
	mkdir -p "$(dirname -- "${target}")"
	if [ -f "${source}" ]; then
		cp "${source}" "${target}"
	fi
done

"${ROOT_DIR}/scripts/generate-api-contracts.sh"

changed=0
for file in "${FILES[@]}"; do
	before="${TMP_DIR}/${file}"
	after="${ROOT_DIR}/${file}"
	if [ ! -f "${before}" ] || ! cmp -s "${before}" "${after}"; then
		changed=1
	fi
done

if [ "${changed}" -ne 0 ]; then
	echo "Generated API contract files were updated."
	echo "Review and commit the generated files, then run make openapi-check-dev again."
	git -C "${ROOT_DIR}" --no-pager diff -- "${FILES[@]}"
	exit 1
fi

echo "Generated API contract files are up to date."
