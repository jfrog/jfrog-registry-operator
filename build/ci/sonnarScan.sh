#!/usr/bin/env bash

set -e

export PROJECT_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd ../../.. && pwd)"
source "${PROJECT_ROOT_DIR}/.jfrog-pipelines/build/ci/scripts/runSonarScannerGeneric.sh"

SONAR_PROJECT_BASE_DIR="${SONAR_PROJECT_BASE_DIR:-}"

SONAR_INCLUSIONS="${SONAR_INCLUSIONS-**/*.go}"
SONAR_EXCLUSIONS="${SONAR_EXCLUSIONS-**/*_test.go}"

function on_start() {
    return 0
}

function on_execute() {
    pushd "${PROJECT_ROOT_DIR}/${SONAR_PROJECT_BASE_DIR}" && ls -lsa
    sonar_runScan
    popd > /dev/null
}

function on_success() {
    return 0
}

function on_complete() {
    return 0
}
