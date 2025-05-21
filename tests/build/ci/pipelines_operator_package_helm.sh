#!/usr/bin/env bash

set -e
export PROJECT_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd ../.. && pwd)"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/pipelineUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/helmUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/commonUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/mavenUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/buildInfoUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/genericRepoUtils.sh"

declare branch
declare helmVirtualRepository
declare dockerVirtualRepository
declare releaseMilestoneBranchFlag
declare dockerImageTag
declare finalChartVersion

function on_start() {
    source "${PROJECT_ROOT_DIR}/build/ci/helmCommon.sh"
    cd "${PROJECT_ROOT_DIR}"
    buildInfo_setBuildNameAndNumber
    branch="$(pipeline_gitBranchName)"
    helmVirtualRepository="$(helmDevArtifactRepoName ${branch})"
    dockerVirtualRepository="$(docker_devArtifactRepoName ${branch})"
    # lintCharts
    chartName="jfrog-registry-operator"
    cd charts/jfrog-registry-operator
    echo $helmVirtualRepository
    echo $dockerVirtualRepository
    echo $branch
    chartVersion=$(grep "version:" "Chart.yaml" | cut -d" " -f 2)
    echo "==> Chart Version : ${chartVersion}"
    
    if common_isReleaseOrMilestoneBranch "${branch}"; then
        releaseMilestoneBranchFlag="true"
    else
        releaseMilestoneBranchFlag="false"
    fi
    dockerImageTag=${res_artifactory_secrets_rotator_operator_entplus_jfrog_io_docker_image_imageTag}

    if [[ "${releaseMilestoneBranchFlag}" != "true" ]]; then
        finalChartVersion="${chartVersion}-x-${run_number}"
    elif [[ ${dockerImageTag} == *m0* ]]; then
        finalChartVersion="${dockerImageTag}"
    else
        finalChartVersion="${chartVersion}"
    fi
}

function on_execute() {    
    UBI_MINIMAL_VERSION="9.2.484"
    packageCharts "${dockerImageTag}" "${releaseMilestoneBranchFlag}" "${UBI_MINIMAL_VERSION}" "${dockerVirtualRepository}"
}

function on_success() {
    uploadCharts "${helmVirtualRepository}" "${JFROG_CLI_BUILD_NAME}" "${JFROG_CLI_BUILD_NUMBER}"
    echoInfo "JFrog Registry Operator helm chart was uploaded to ${helmVirtualRepository}"
}

function on_failure() {
    errorExit "Helm chart packaging failed..."
}

function on_complete() {
    buildInfo_publish
    write_output artifactory_secrets_rotator_operator_helm_chart_build_info \
        buildName="${JFROG_CLI_BUILD_NAME}" \
        buildNumber="${JFROG_CLI_BUILD_NUMBER}" \
        version="${finalChartVersion}"
}

function errorExit() {
    echoInfo "[ERROR]" "$@"
    exit 1
}

function echoInfo() {
    echo "[JFrog Registry Operator Helm chart]" "$@"
}