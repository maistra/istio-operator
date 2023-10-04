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

# To run this integration test on OCP cluster it's needed to already have the OCP cluster running and be logged in

# Deploy Operator
echo "Deploying Operator"
cd "$(git rev-parse --show-toplevel)" && make deploy

# Run the integration tests
echo "Running integration tests"
./tests/integration/common-operator-integ-suite.sh --ocp