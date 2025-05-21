#!/usr/bin/env bash

set -ex

export PROJECT_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd ../.. && pwd)"
echo "project root dir in packageDocker: ${PROJECT_ROOT_DIR}"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/commonUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/mavenUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/dockerUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/versionUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/pipelineUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/buildInfoUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/goUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/genericRepoUtils.sh"


export OPERATOR_ROOT_DIR="${PROJECT_ROOT_DIR}"


declare moduleVersion
declare binaryVersion

declare version
declare registry
declare branch
declare dockerVirtualRepository
declare normalizedBranch
declare targetVersion
declare fullImageNameCron
declare fullImageName



function on_start() {

    cd "${PROJECT_ROOT_DIR}"
    "${PROJECT_ROOT_DIR}/build/ci/scripts/goPrepare.sh" && export_run_variables
    useEmbeddedJfrogCli
    buildInfo_setBuildNameAndNumber

    mkdir -p $HOME/.docker/cli-plugins
    jfrog rt dl --flat=false common-docker-local/buildx-v0.9.1.linux-amd64 $HOME/.docker/cli-plugins/docker-buildx
    chmod +x $HOME/.docker/cli-plugins/docker-buildx

    docker --version
    docker buildx version
    curl -fL https://install-cli.jfrog.io | sh
    jf --version

    branch="$(pipeline_gitBranchName)"
    dockerVirtualRepository="$(docker_devArtifactRepoName ${branch})"
    registry="entplus.jfrog.io"
    echoInfo "Git Branch: $branch"

    local moduleName
    moduleName="$(getGoModuleName "./go.mod")"
    echoInfo "Module Name: $moduleName"
    originalModuleVersion="$(go_getGoModuleVersionFromEnv "./go.mod")"
    echo "originalModuleVersion:$originalModuleVersion"
    # version template is 0.0.0-<branch_name>-<timestamp>-<commit_sha>
    # removing the timestamp to make it shorter
    withoutShaAndTimestamp=$(echo $originalModuleVersion | rev | cut -d- -f3- | rev)
    sha=$(echo $originalModuleVersion | rev | cut -d- -f1 | rev)
    moduleVersion="${withoutShaAndTimestamp}-${sha}"
    if version_isRelease "${originalModuleVersion}" || version_isMilestone "${originalModuleVersion}"; then
        moduleVersion="${originalModuleVersion}"
    fi
    echoInfo "Version: $moduleVersion"
    binaryVersion="${moduleVersion}-${run_number}"
    echoInfo "Binary Version: $binaryVersion"
    local revision="$(pipeline_gitCommitSha)"
    echoInfo "Revision: $revision"

    make operator
}



function on_execute() {
    cd "${OPERATOR_ROOT_DIR}"
    version="$(maven_getVersionFromPom "pom.xml")"
    normalizedBranch=$(common_normalizeBranchName "${branch}")
    targetVersion="${version}-${normalizedBranch}-${run_number}"

    if version_isRelease "${version}" || version_isMilestone "${version}"; then
        targetVersion="${version}"
    fi
    echo "targetVersion=${targetVersion}"

    echo "======== Building and pushing a multiarch image through  buildx ========"
    fullImageName="${registry}/${dockerVirtualRepository}/jfrog/jfrog-registry-operator:${targetVersion}"
    make build-operator-docker OPERATOR_IMAGE_NAME="${fullImageName}"
}

function on_success() {
    OPERATOR_DOCKER_BUILD_NAME="$(docker_generateBuildName)"
    write_output artifactory_secrets_rotator_operator_docker_build_info \
        buildName="${OPERATOR_DOCKER_BUILD_NAME}" \
        buildNumber="${JFROG_CLI_BUILD_NUMBER}" \
        version="$targetVersion"
    write_output artifactory_secrets_rotator_operator_entplus_jfrog_io_docker_image sourceRepository="${dockerVirtualRepository}" imageName="${dockerVirtualRepository}/inst-jfrog-registry-operator/inst-jfrog-registry-operator" imageTag="${targetVersion}"
}

function on_failure() {
    return 0
}

function on_complete() {
    return 0
}

function echoInfo() {
    echo "[JFrog Registry Operator docker]" "$@"
}

function useEmbeddedJfrogCli() {
    echo "[INFO] Using Jfrog CLI embedded in /usr/bin"
    export PATH=/usr/bin:${PATH}
    echo "[INFO] Checking Jfrog CLI version on PATH"
    jfrog --version
}