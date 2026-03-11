#!/usr/bin/env bash
# Transpiration E2E: (1) optional ingest of sample MD, (2) verify_transpiration.py keyword search.
# Test environment: docker (use .env with Docker internal hostnames or port-mapped localhost).
set -e
cd "$(dirname "$0")/.."

[[ -f .env ]] || { echo "No .env" >&2; exit 1; }
set -a
# shellcheck source=/dev/null
source ./.env
set +a

SAMPLE_MD="${1:-tests/fixtures/sample.md}"
KEYWORD="${2:-Introduction}"

echo "=== Transpiration E2E (sample=${SAMPLE_MD}, keyword=${KEYWORD}) ==="

# 1) Ingest sample markdown so Phloem writes to PG + Qdrant, and Root syncs to TypeDB if TYPEDB_HOST set
if [[ -f "$SAMPLE_MD" ]]; then
  echo "Ingesting $SAMPLE_MD ..."
  python -m property.root_props.run "$SAMPLE_MD" || { echo "Ingest failed" >&2; exit 2; }
else
  echo "No sample file at $SAMPLE_MD; skipping ingest. Ensure data exists in PG/Qdrant/TypeDB for search."
fi

# 2) Transpiration: keyword -> Qdrant search -> TypeDB section context
echo "Running verify_transpiration.py \"$KEYWORD\" ..."
python scripts/verify_transpiration.py "$KEYWORD"
exitcode=$?

if [[ $exitcode -eq 0 ]]; then
  echo "=== Transpiration E2E done (OK) ==="
else
  echo "=== Transpiration E2E failed (exit $exitcode) ===" >&2
fi
exit $exitcode
