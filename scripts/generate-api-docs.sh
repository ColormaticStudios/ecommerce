#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC_FILE="${ROOT_DIR}/api/openapi.yaml"
OUTPUT_FILE="${ROOT_DIR}/API.md"
TMP_FILE="${ROOT_DIR}/API.md.tmp"

if ! command -v bunx >/dev/null 2>&1; then
	echo "bunx is required to generate API docs"
	exit 1
fi

# In restricted environments bun may not be able to write to the default install/tmp locations.
export BUN_INSTALL="${BUN_INSTALL:-/tmp/bun}"
export BUN_TMPDIR="${BUN_TMPDIR:-/tmp}"

# Generate Markdown using JavaScript-only code samples.
bunx widdershins "${SPEC_FILE}" \
	--summary \
	--omitHeader \
	--language_tabs "javascript:JavaScript" \
	-o "${TMP_FILE}"

# Keep the docs compact:
# 1. Remove generated example response payloads (response tables are retained).
# 2. Drop the giant schema appendix (the canonical schema is api/openapi.yaml).
awk '
BEGIN { skip_examples = 0; skip_schemas = 0 }
/^> Example responses$/ { skip_examples = 1; next }
/^<h3 id="[^"]*-responses">Responses<\/h3>$/ { skip_examples = 0 }
/^# Schemas$/ { skip_schemas = 1; next }
!skip_examples && !skip_schemas { print }
' "${TMP_FILE}" \
| sed -E 's/\[([A-Za-z0-9_]+)\]\(#schema[A-Za-z0-9_]+\)/\1/g' \
> "${OUTPUT_FILE}"

rm -f "${TMP_FILE}"

echo "Generated API docs:"
echo " - API.md"
