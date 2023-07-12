#!/bin/bash


# Copyright 2023 Red Hat, Inc.

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Exit immediately for non zero status
set -e
# Check unset variables
set -u
# Print commands
set -x

WD=$(dirname "$0")
WD=$(cd "$WD"; pwd)

# build and push operator image
build_operator_image() { # compile operator source and push operator image
    local TAG="${TAG:-$(git rev-parse HEAD)}"
    local BUILDER_CLI="${BUILDER_CLI:-docker}"

    # var name IMAGE is in kind provisioner
    export OPERATOR_IMAGE="${OPERATOR_IMAGE:-localhost:5000/istio-operator-integ}:${TAG}"

    echo "build and push operator image..."
    echo
    cd "$(git rev-parse --show-toplevel)"
    make gen build

    timeout --foreground -v -s SIGHUP -k 5m 5m bash --verbose -c \
      "until make docker-build; do sleep 5; done"
    
    # Retry due to push failures that randomly occur
    timeout --foreground -v -s SIGHUP -k 5m 5m bash --verbose -c \
      "until make docker-push; do sleep 5; done"

    cd "$WD"
}

build_operator_image