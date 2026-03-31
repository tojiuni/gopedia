#!/usr/bin/env bash
# tag-release.sh — create a signed git tag + GitHub Release for Gopedia.
#
# Usage:
#   ./scripts/tag-release.sh v0.2.0 "IMP-01 duplicate prevention, IMP-02 source_path"
#
# Prerequisites:
#   - gh CLI authenticated (gh auth login)
#   - On main branch with a clean working tree
#   - CHANGELOG.md updated with the new version section
#
# What it does:
#   1. Validates semver tag format (vX.Y.Z)
#   2. Checks clean working tree and main branch
#   3. Creates annotated git tag
#   4. Pushes tag to origin
#   5. Creates GitHub Release with CHANGELOG section as body
#   6. Prints next-steps reminder for RAG test report

set -euo pipefail

# ── Args ──────────────────────────────────────────────────────────────────────
VERSION="${1:-}"
SUMMARY="${2:-}"

if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 <vX.Y.Z> [\"short summary\"]" >&2
  exit 1
fi

if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: version must match vX.Y.Z (e.g. v0.2.0)" >&2
  exit 1
fi

# ── Checks ────────────────────────────────────────────────────────────────────
BRANCH=$(git branch --show-current)
if [[ "$BRANCH" != "main" ]]; then
  echo "Error: must be on 'main' branch (currently on '$BRANCH')" >&2
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Error: working tree is not clean. Commit or stash changes first." >&2
  exit 1
fi

if git tag --list | grep -q "^${VERSION}$"; then
  echo "Error: tag $VERSION already exists." >&2
  exit 1
fi

# ── Extract CHANGELOG section ─────────────────────────────────────────────────
NOTES=""
if [[ -f CHANGELOG.md ]]; then
  # Extract block between ## [VERSION] and next ## [
  NOTES=$(awk "/^## \[${VERSION//./\\.}\]/,/^## \[/" CHANGELOG.md \
    | head -n -1 | tail -n +2 || true)
fi

if [[ -z "$NOTES" ]]; then
  NOTES="${SUMMARY:-No changelog entry found for ${VERSION}.}"
fi

# ── Tag ───────────────────────────────────────────────────────────────────────
TAG_MSG="${VERSION}: ${SUMMARY:-$(echo "$NOTES" | head -1)}"
echo "Creating annotated tag: $VERSION"
git tag "$VERSION" -m "$TAG_MSG"
git push origin "$VERSION"
echo "Tag pushed: $VERSION"

# ── GitHub Release ────────────────────────────────────────────────────────────
if command -v gh &>/dev/null; then
  RELEASE_BODY="${NOTES}

---
See [CHANGELOG.md](CHANGELOG.md) for full details."

  gh release create "$VERSION" \
    --title "$VERSION — ${SUMMARY:-Gopedia release}" \
    --notes "$RELEASE_BODY"
  echo "GitHub Release created: $VERSION"
else
  echo "gh CLI not found — skipping GitHub Release creation."
  echo "Run manually: gh release create $VERSION --title '...' --notes '...'"
fi

# ── Reminder ──────────────────────────────────────────────────────────────────
DATE=$(date +%Y-%m-%d)
echo ""
echo "Next steps:"
echo "  1. Run RAG quality test and save report:"
echo "     doc/rag-test-reports/${VERSION}_${DATE}_<target>.md"
echo "  2. Update doc/rag-test-reports/README.md table."
echo "  3. Update IMPROVEMENTS.md — mark resolved items."
