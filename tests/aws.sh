#!/bin/bash

set -x
set -e

echo "Artifact dir = ${ARTIFACTS}"

echo success > "${ARTIFACTS}/results.txt"
