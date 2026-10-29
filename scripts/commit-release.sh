#!/usr/bin/env bash
set -euo pipefail

: "${RELEASE_BRANCH:?RELEASE_BRANCH is required}"
: "${RELEASE_TAG:?RELEASE_TAG is required}"
: "${BASE_SHA:?BASE_SHA is required}"

if [[ ! "$RELEASE_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || [[ ! "$BASE_SHA" =~ ^[0-9a-f]{40}$ ]]; then
    echo "Invalid release tag or source commit." >&2
    exit 1
fi
if [[ "$(git rev-parse HEAD)" != "$BASE_SHA" ]]; then
    echo "Checkout must match the commit used to build this release." >&2
    exit 1
fi

git add -- plugin.json webapp/package.json webapp/package-lock.json
expected_tree="$(git write-tree)"

# Retrying a failed publish job must reuse the exact already-pushed release,
# never move a tag or create a second version commit.
if git show-ref --verify --quiet "refs/tags/$RELEASE_TAG"; then
    release_commit="$(git rev-parse "$RELEASE_TAG^{commit}")"
    release_parent="$(git rev-parse --verify "$release_commit^" 2>/dev/null || true)"
    if [[ "$(git rev-parse "$release_commit^{tree}")" != "$expected_tree" ]] || [[ "$release_parent" != "$BASE_SHA" ]]; then
        echo "Existing release tag belongs to different source; refusing to reuse it." >&2
        exit 1
    fi
    if ! remote_tag="$(git ls-remote --exit-code origin "refs/tags/$RELEASE_TAG")" || [[ "${remote_tag%%$'\t'*}" != "$release_commit" ]]; then
        echo "Existing local tag is not the matching published remote tag." >&2
        exit 1
    fi
else
    if git diff --cached --quiet; then
        echo "No version change to commit." >&2
        exit 1
    fi
    git config user.name 'github-actions[bot]'
    git config user.email '41898282+github-actions[bot]@users.noreply.github.com'
    git commit -m "Release $RELEASE_TAG"
    git tag -- "$RELEASE_TAG"
    # A concurrent branch update or a protected branch rejects BOTH updates.
    # No force push and no bypass of repository protections is attempted.
    git push --atomic origin "HEAD:refs/heads/$RELEASE_BRANCH" "refs/tags/$RELEASE_TAG"
    release_commit="$(git rev-parse HEAD)"
fi

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    printf 'commit=%s\n' "$release_commit" >> "$GITHUB_OUTPUT"
fi
printf 'Release %s uses commit %s\n' "$RELEASE_TAG" "$release_commit"
