#!/bin/env bash

function removeOldVersions() {
    versions=$(yq eval '.versions | keys | .[]' versions.yaml | tr $'\n' ' ')
    for subdirectory in resources/*/; do
        version=$(basename "$subdirectory")
        if [[ ! " ${versions} " == *" $version "* ]]; then
            echo "Removing: $subdirectory"
            rm -r "$subdirectory"
        fi
    done
}

removeOldVersions
