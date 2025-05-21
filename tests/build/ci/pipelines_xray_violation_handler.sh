#!/usr/bin/env bash

set -e

export PROJECT_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd ../.. && pwd)"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/commonUtils.sh"

function run_violation_handler() {
    local branch="$(pipeline_gitBranchName)"
    local buildName="${1}"
    local buildNumber="${2}"
    local serviceName="jfrog-registry-operator"

    if [[ -z "$buildName" ]]; then
        echo "[ERROR] run_violation_handler: buildName argument is mandatory"
        exit 1
    fi

    if [[ -z "$buildNumber" ]]; then
        echo "[ERROR] run_violation_handler: buildNumber argument is mandatory"
        exit 1
    fi

    docker login "${int_entplus_deployer_url}" \
        -u "${int_entplus_deployer_user}" -p "${int_entplus_deployer_apikey}"

    docker run \
        -e XVH_JPD_TOKEN="${int_security_xray_access_token_accessToken}" \
        -e XVH_JIRA_USERNAME="${int_jira_jfrog_username}" \
        -e XVH_JIRA_TOKEN="${int_jira_jfrog_token}" \
        entplus.jfrog.io/qa-docker/xray-violation-handler:latest "${serviceName}" -b "${buildName}" -n "${buildNumber}" -i
}
