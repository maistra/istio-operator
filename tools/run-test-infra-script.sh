#!/bin/bash

# Copyright Maistra Authors. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# This script blindly forwards whatever is passed to its command line to the maistra/test-infra repository.
# Its main purpose is to run a postsubmit job in OpenShift CI when the job configuration cannot rely on a external repository.
# So, this script just clones the test-infra repo and runs whatever is passed to the command line, including the command itself (1st argument)
#
# Example of usage:
# ./maistra/run-test-infra-script.sh ./tools/automator.sh \
#   -o maistra \
#   -r proxy \
#   -b maistra-2.3
#
# Note in the example above the first argument is the actual script that's going to be invoked in the test-infra repository.
# The "./tools/automator.sh" command above is relative to the test-infra repository tree.
#
# See the global variable definitions below if you are interested in running this locally pointing to a local test-infra directory.

set -eux -o pipefail

TEST_INFRA_REPO="${TEST_INFRA_REPO:-https://github.com/maistra/test-infra.git}"
TEST_INFRA_BRANCH="${TEST_INFRA_BRANCH:-main}"
SKIP_CLEANUP="${SKIP_CLEANUP:-}"

function cleanup() {
  if [ -z "${SKIP_CLEANUP:-}" ]; then
    rm -rf "${TEST_INFRA_LOCAL_DIR:-}"
  fi
}

trap cleanup EXIT

if [ -z "${TEST_INFRA_LOCAL_DIR:-}" ]; then
  TEST_INFRA_LOCAL_DIR=$(mktemp -d)
  git clone --single-branch --depth=1 -b "${TEST_INFRA_BRANCH}" "${TEST_INFRA_REPO}" "${TEST_INFRA_LOCAL_DIR}"
else
  SKIP_CLEANUP="true"
fi

cd "${TEST_INFRA_LOCAL_DIR}"

# Run everything that's passed on the command line, including the command itself (1st argument)
"$@"