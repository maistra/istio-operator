#!/bin/bash

set -euo pipefail

check_watches() {
    # Find kinds in charts
    read -r -a chartKinds <<< "$(grep -rEo "^kind: ([A-Za-z0-9]+)" --no-filename ./resources/*/charts | sed -e 's/^kind: //g' | sort | uniq | tr '\n' ' ')"
    echo "Kinds in charts: ${chartKinds[*]}"

    # Find watched kinds in istio_controller.go
    read -r -a watchedKinds <<< "$(grep -Eo "(Owns|Watches)\\((.*)" ./controllers/istio_controller.go | sed 's/.*&[^.]*\.\([^{}]*\).*/\1/' | sort | uniq | tr '\n' ' ')"
    echo "Watched kinds: ${watchedKinds[*]}"

    # Find ignored kinds in istio_controller.go
    read -r -a ignoredKinds <<< "$(sed -n 's/.*\+lint-watches:ignore:\s*\(\w*\).*/\1/p' ./controllers/istio_controller.go | sort | uniq | tr '\n' ' ')"
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
        printf "The following kinds aren't watched in istio_controller.go:\n"
        for line in "${missing_kinds[@]}"; do
            printf "  - %s\n" "$line"
        done
        exit 1
    else
        printf "Controller watches all kinds found in Helm charts.\n"
    fi
}

check_watches
