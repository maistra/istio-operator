#!/usr/bin/env bash

function sed() {
  echo "ERROR: detected direct sed invocation"
  echo "Please use sed_wrap. It is a wrapper around sed that fails when no changes have been detected."
  echo "Failed call was: sed $@"
  return 1
}

function sed_wrap() {
  for filename; do true; done # this retrieves the last argument
  echo "patching $filename"
  state=$(cat $filename)
  command sed "$@"
  difference=$(diff <(echo "${state}") <(cat ${filename}) || true )
  if [[ -z "${difference}" ]]; then
    echo "ERROR: nothing changed, sed seems to not have matched. Exiting"
    echo "Failed call: sed $*"
    return 10
  fi
}
