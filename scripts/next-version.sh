#!/bin/sh
# Prints the version the next release gets, from the newest v* tag and the
# commits since it. Prints nothing when there is nothing new to release.
#
#   breaking change ("feat!:", "BREAKING CHANGE:" or "[major]")  major
#   new feature ("feat:" or "feat(scope):", or "[minor]")         minor
#   anything else                                                 patch
#
# Before 1.0.0 a breaking change only bumps the minor version. The first
# release is 0.1.0.
set -eu

last="$(git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' --exclude '*-*' 2>/dev/null || true)"
if [ -z "$last" ]; then
    echo 0.1.0
    exit 0
fi
messages="$(git log --format=%B "$last..HEAD")"
[ -n "$messages" ] || exit 0

version="${last#v}"
version="${version%%-*}"
major="${version%%.*}"
rest="${version#*.}"
minor="${rest%%.*}"
patch="${rest#*.}"

if printf '%s\n' "$messages" | grep -qE '^[a-z]+(\([^)]*\))?!:|^BREAKING[ -]CHANGE:|\[major\]'; then
    if [ "$major" -eq 0 ]; then
        minor=$((minor + 1)) patch=0
    else
        major=$((major + 1)) minor=0 patch=0
    fi
elif printf '%s\n' "$messages" | grep -qE '^feat(\([^)]*\))?:|\[minor\]'; then
    minor=$((minor + 1)) patch=0
else
    patch=$((patch + 1))
fi
echo "$major.$minor.$patch"
