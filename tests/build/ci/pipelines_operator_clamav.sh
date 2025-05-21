#!/usr/bin/env bash

set -e

export PROJECT_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd ../.. && pwd)"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/pipelineUtils.sh"

# Choose AV Version by setting the tag of the image
branch="$(pipeline_gitBranchName)"
registry="$(pipeline_imageRegistry "${imageResourceName}")"
repository=$(common_devArtifactRepoName "$branch" "docker" "local")
imageName="$(pipeline_imageName "${imageResourceName}")"
imageTag="$(pipeline_imageTag "${imageResourceName}")"

export CLAM_IMAGE_TAG="latest"
export CLAM_DOCKER_IMAGE="${registry}/${repository}/jfrog/jfrog-registry-operator:${imageTag}"
export CLAM_MOUNT_PATH="/operator"

function clam_execute() {
    docker-compose -p operator -f build/ci/docker-compose-clamav.yml up -V --no-build --exit-code-from clamscanner
}

function clam_complete() {
    docker-compose -p operator -f build/ci/docker-compose-clamav.yml down --volumes
}

#!/bin/bash

export PROJECT_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd ../../../.. && pwd)"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/commonUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/mavenUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/dockerUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/versionUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/pipelineUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/buildInfoUtils.sh"

CLAMAV_SCAN_HOME="$(cd "$(dirname "${BASH_SOURCE[0]}")/" && pwd)"
CLAMAV_SCAN_DIR=${CLAMAV_SCAN_HOME}/clamav-scan-dir
CLAMAV_SCAN_PRODUCT_NAME=artifactory-pro
CLAMAV_SCAN_COMPOSE="./docker-compose-clamav-installers.yml"

function errorExit() {
    echo; echo -e "ERROR:$1"; echo
    exit 1
}

function errorExitZero() {
    echo; echo -e "ERROR:$1"; echo
    exit 0
}

function createScanFileList(){
    local downloadVersion=${CLAMAV_SCAN_PRODUCT_VERSION//SNAPSHOT/*}
    case "${step_name}" in
        *compose* )
            CLAMAV_SCAN_FILE_LIST="jfrog-${CLAMAV_SCAN_PRODUCT_NAME}-${downloadVersion}-compose.tar.gz" ;;
        *rpm* )
            CLAMAV_SCAN_FILE_LIST="jfrog-${CLAMAV_SCAN_PRODUCT_NAME}-${downloadVersion}.rpm" ;;
        *deb* )
            CLAMAV_SCAN_FILE_LIST="jfrog-${CLAMAV_SCAN_PRODUCT_NAME}-${downloadVersion}.deb" ;;
        * )
            CLAMAV_SCAN_FILE_LIST="jfrog-${CLAMAV_SCAN_PRODUCT_NAME}-${downloadVersion}-linux.tar.gz"
            CLAMAV_SCAN_FILE_LIST=" ${CLAMAV_SCAN_FILE_LIST} jfrog-${CLAMAV_SCAN_PRODUCT_NAME}-${downloadVersion}-darwin.tar.gz"
            CLAMAV_SCAN_FILE_LIST=" ${CLAMAV_SCAN_FILE_LIST} jfrog-${CLAMAV_SCAN_PRODUCT_NAME}-${downloadVersion}-windows.zip"
            ;;
    esac
}

function setup() {
    echo; echo "CLAMAV_SCAN_HOME=$CLAMAV_SCAN_HOME"
    echo "CLAMAV_SCAN_DIR=$CLAMAV_SCAN_DIR"

    echo; echo "Cleaning up old packages"
    if [ -d "${CLAMAV_SCAN_DIR}" ]; then
        rm -rf ${CLAMAV_SCAN_DIR}
    fi
    mkdir -p ${CLAMAV_SCAN_DIR} || errorExit "Failed to create CLAMAV_SCAN_DIR:$CLAMAV_SCAN_DIR"
}

function downloadFileUsingCLI {
  local file=$1
  local dest=$2
  local extract=${3:-false}
  local flat=${4:-true}
    echo -e "Attempting download of ${file}..."
    export JFROG_CLI_OFFER_CONFIG=false
	${CLAMAV_SCAN_HOME}/jfrog rt dl "${file}" "${dest}" --flat=${flat} --limit=1 --sort-by=created --sort-order=desc --explode=${extract} --url="https://entplus.jfrog.io/artifactory" --user=${int_entplus_deployer_user} --password=${int_entplus_deployer_apikey} || errorExitZero "Download failed : ${file}"
}

function downloadFiles () {
    local fileList="$1"
    local destination="$2"
	local extract=false
	local flat=true

    for file in ${fileList}; do
        downloadFileUsingCLI "${CLAMAV_SCAN_DOWNLOAD_REPO}/*/${file}" "${destination}"/
    done
}

function installJFrogCLI() {
    if [ ! -f ${CLAMAV_SCAN_HOME}/jfrog ]; then
        mkdir -p ${CLAMAV_SCAN_HOME}
        cd ${CLAMAV_SCAN_HOME}

        echo -e "Installing JFrog CLI"
        curl -fL https://getcli.jfrog.io | sh -s "1.41.2" || errorExit "Installing JFrog CLI failed"
    fi
}

function clamComposeUp() {
    docker-compose -p ${CLAMAV_SCAN_PRODUCT_NAME} -f ${CLAMAV_SCAN_COMPOSE} up -V --no-build --exit-code-from clamscanner
}

function clamComposeDown() {
    echo -e "Cleaning up containers on ${CLAMAV_SCAN_PRODUCT_NAME} project"
    docker-compose -p ${CLAMAV_SCAN_PRODUCT_NAME} -f ${CLAMAV_SCAN_COMPOSE} down --volumes
}

function scanDir() {
    export CLAMAV_SCAN_DIR="${CLAMAV_SCAN_DIR}"
    export CLAM_IMAGE_TAG=${CLAM_IMAGE_TAG}

    echo -e "\nInitiating scan on: \n\t $(ls ${CLAMAV_SCAN_DIR}/*)\n\n"
    clamComposeUp || errorExit "Scan on ${CLAMAV_SCAN_DIR} resulted in failure"
    clamComposeDown
}

function clam_execute() {
    export CLAMAV_SCAN_PRODUCT_VERSION="$(maven_getVersionFromPom "${PROJECT_ROOT_DIR}/artifactory-product/artifactory-pro/pom.xml")"
    local branch
    branch="$(pipeline_gitBranchName)"
    export CLAMAV_SCAN_DOWNLOAD_REPO="$(maven_devArtifactRepoName "$branch")"
    export INSTALLER_TYPE="all"
    export CLAM_IMAGE_TAG="latest"
    createScanFileList
    installJFrogCLI
    downloadFiles "${CLAMAV_SCAN_FILE_LIST}" "${CLAMAV_SCAN_DIR}"
    scanDir "${CLAMAV_SCAN_DIR}"
}

function clam_complete() {
    echo "Scan complete"
}