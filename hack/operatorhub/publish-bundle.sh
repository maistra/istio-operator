#!/bin/bash
# shellcheck disable=SC1091

set -euo pipefail

show_help() {
  echo "publish-bundle - raises PR to Operator Hub"
  echo " "
  echo "./publish-bundle.sh [options]"
  echo " "
  echo "Options:"
  echo "-h, --help        shows brief help"
  echo "-d, --dry-run     skips push to GH and PR"
}

dryRun=false

skipInDryRun() {
  if $dryRun; then echo "# $*";  fi
  if ! $dryRun; then "$@";  fi
}

while test $# -gt 0; do
  case "$1" in
    -h|--help)
            show_help
            exit 0
            ;;
    -d|--dry-run)
            dryRun=true
            shift
            ;;
    *)
            echo "Unknown param $1"
            exit 1
            ;;
  esac
done

CUR_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
BUNDLE_DIR="${CUR_DIR}"/../../bundle/

GITHUB_TOKEN="${GITHUB_TOKEN:-}"
GIT_USER="${GIT_USER:-}"

# The OPERATOR_NAME is defined in Makefile
: "${OPERATOR_NAME:?"Missing OPERATOR_NAME variable"}"
: "${OPERATOR_VERSION:?"Missing OPERATOR_VERSION variable"}"
OPERATOR_HUB=${OPERATOR_HUB:-"community-operators-prod"}

TMP_DIR=$(mktemp -d -t "${OPERATOR_NAME}.XXXXXXXXXX")
trap '{ rm -rf -- "$TMP_DIR"; }' EXIT

OWNER="${OWNER:-"redhat-openshift-ecosystem"}"
HUB_REPO_URL="${HUB_REPO_URL:-https://github.com/${OWNER}/${OPERATOR_HUB}.git}"
HUB_BASE_BRANCH="${HUB_BASE_BRANCH:-main}"

FORK="${FORK:-maistra}"
FORK_REPO_URL="${FORK_REPO_URL:-https://${GIT_USER}:${GITHUB_TOKEN}@github.com/${FORK}/${OPERATOR_HUB}.git}"

BRANCH=${BRANCH:-"${OPERATOR_NAME}-${OPERATOR_VERSION}"}

source "${CUR_DIR}"/../validate_semver.sh

validate_semantic_versioning "v${OPERATOR_VERSION}"

git clone "${HUB_REPO_URL}" "${TMP_DIR}"

cd "${TMP_DIR}"
git remote add fork "${FORK_REPO_URL}"
skipInDryRun git push fork "${HUB_BASE_BRANCH}" # ensures our fork is in sync with upstream
git checkout -b "${BRANCH}"

OPERATORS_DIR="operators/${OPERATOR_NAME}/${OPERATOR_VERSION}/"
mkdir -p "${OPERATORS_DIR}"
cp -a "${BUNDLE_DIR}"/. "${OPERATORS_DIR}"

TITLE="operators ${OPERATOR_NAME} (${OPERATOR_VERSION})"
skipInDryRun git add .
skipInDryRun git commit -s -m"${TITLE}"

if [[ ! $dryRun && -z $GITHUB_TOKEN ]]; then
  echo "Please provide GITHUB_TOKEN" && exit 1
fi

skipInDryRun git push fork "${BRANCH}"

PAYLOAD=$(mktemp)

jq -c -n \
  --arg msg "$(cat "${CUR_DIR}"/operatorhub-pr-template.md)" \
  --arg head "${FORK}:${BRANCH}" \
  --arg base "${HUB_BASE_BRANCH}" \
  --arg title "${TITLE}" \
   '{head: $head, base: $base, title: $title, body: $msg }' > "${PAYLOAD}"

if $dryRun; then
  echo -e "${PAYLOAD}\n------------------"
  jq . "${PAYLOAD}"
fi

skipInDryRun curl \
  -X POST \
  -H "Authorization: token ${GITHUB_TOKEN}" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/"${OWNER}/${OPERATOR_HUB}"/pulls \
   --data-binary "@${PAYLOAD}"
