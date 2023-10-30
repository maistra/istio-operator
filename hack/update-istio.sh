#!/usr/bin/env bash

set -euo pipefail

COMMIT=$(yq eval '"git ls-remote --heads " + .latest.repo + ".git " + .latest.branch + " | cut -f 1"' versions.yaml | sh)
echo Updating version 'latest' to commit ${COMMIT}

yq -i '.latest.commit="'${COMMIT}'"' versions.yaml
