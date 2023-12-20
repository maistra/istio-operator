#!/bin/bash

# Copyright Istio Authors
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

set -euo pipefail

check_watches() {
    # Find kinds in charts
    read -r -a chartKinds <<< "$(grep -rEo "^kind: ([A-Za-z0-9]+)" --no-filename ./resources/*/charts | sed -e 's/^kind: //g' | sort | uniq | tr '\n' ' ')"
    echo "Kinds in charts: ${chartKinds[*]}"

    # Find watched kinds in istiorevision_controller.go
    read -r -a watchedKinds <<< "$(grep -Eo "(Owns|Watches)\\((.*)" ./controllers/istiorevision/istiorevision_controller.go | sed 's/.*&[^.]*\.\([^{}]*\).*/\1/' | sort | uniq | tr '\n' ' ')"
    echo "Watched kinds: ${watchedKinds[*]}"

    # Find ignored kinds in istiorevision_controller.go
    read -r -a ignoredKinds <<< "$(sed -n 's/.*\+lint-watches:ignore:\s*\(\w*\).*/\1/p' ./controllers/istiorevision/istiorevision_controller.go | sort | uniq | tr '\n' ' ')"
    echo "Ignored kinds: ${ignoredKinds[*]}"

    # Check for missing lines
    local missing_kinds=()
    for kind in "${chartKinds[@]}"; do
        # shellcheck disable=SC2076
        if [[ ! " ${watchedKinds[*]} ${ignoredKinds[*]} " =~ " $kind " ]]; then
            missing_kinds+=("$kind")
        fi
    done

    # Print missing lines, if any
    if [[ ${#missing_kinds[@]} -gt 0 ]]; then
        printf "The following kinds aren't watched in istiorevision_controller.go:\n"
        for line in "${missing_kinds[@]}"; do
            printf "  - %s\n" "$line"
        done
        exit 1
    else
        printf "Controller watches all kinds found in Helm charts.\n"
    fi
}

check_watches
