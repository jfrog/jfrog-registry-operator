#!/usr/bin/env bash

set -e

PROJECT_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd ../.. && pwd)"
pushd "${PROJECT_ROOT_DIR}"

# Use the jfdev-ci-commons as a library in the services.
# This script downloads the jfdev-ci-commons npm package and extract the files inside it to the destination folder: build/ci/scripts/
# The service will use the ci code locally.
# In the service add in every step:
# onStart:
# - source "${res_<service>_bitbucket_resourcePath}/build/ci/pipelines_step_setup.sh"
#


JFDEV_CI_COMMONS_DEFAULT_VERSION="1.*"

if [[ -z "${JFDEV_CI_COMMONS_VERSION}" ]]; then
    # Download latest release version (without  milestone + dev versions) based on regex in 'JFDEV_CI_COMMONS_DEFAULT_VERSION'
     # Command breakdown:
     # jq -c '.[].props["npm.version"]' -> Return the value under [props][npm.version]
     # egrep -v -e "-m" -e "-dev" -> Remove the result with -m and -dev so in this case "1.2.0-dev" and "1.8.0-m016" will be removed
     # sort --version-sort -> Return asc sorted data for all version numeric-wise
     # tail -1 -> Return the last output print line since its already sorted

    JFDEV_CI_COMMONS_VERSION=$(jfrog rt search "npm-releases-local/jfdev-ci-commons/-/jfdev-ci-commons-${JFDEV_CI_COMMONS_DEFAULT_VERSION}.tgz" |  jq -cr '.[].props["npm.version"][]' | egrep -v -e "-m" -e "-dev"  | sort --version-sort | tail -1)
    echo "Found JFdev ci commons latest: ${JFDEV_CI_COMMONS_VERSION} "
    jfrog rt dl --explode  "npm-releases-local/jfdev-ci-commons/-/jfdev-ci-commons-${JFDEV_CI_COMMONS_VERSION}.tgz"
else
    # Download specific version user asked for in env var 'JFDEV_CI_COMMONS_VERSION' - could be release/milestone/dev
    jfrog rt dl --explode "*/jfdev-ci-commons/-/jfdev-ci-commons-${JFDEV_CI_COMMONS_VERSION}.tgz"
fi

CI_SCRIPTS_FOLDER="${PROJECT_ROOT_DIR}/.jfrog-pipelines/build/ci/scripts"
echo "${CI_SCRIPTS_FOLDER}"
if [ -d "${CI_SCRIPTS_FOLDER}" ]; then
    echo "${CI_SCRIPTS_FOLDER} exists"
    # Directory ${CI_SCRIPTS_FOLDER} exists
    if [ "$(ls -A ${CI_SCRIPTS_FOLDER})" ]; then
        # Soon this folder will be removed from artifactory to jfdev-ci-commons repository.
        # After the remove this warning will become an error
        echo "[WARN] ${CI_SCRIPTS_FOLDER} is not empty"
        rm -rf "${CI_SCRIPTS_FOLDER}/*"
    fi
fi

mkdir -p "${CI_SCRIPTS_FOLDER}"; cp -Rf jfdev-ci-commons/-/package/* "${CI_SCRIPTS_FOLDER}/"

source "${CI_SCRIPTS_FOLDER}/lib/pipelineStepCommons.sh"