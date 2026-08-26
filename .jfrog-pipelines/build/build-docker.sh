#!/bin/bash

export PROJECT_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/commonUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/pipelineUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/buildInfoUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/dockerUtils.sh"
source "${PROJECT_ROOT_DIR}/build/ci/scripts/lib/mavenUtils.sh"

errorExit () {
    echo; echo "ERROR: $1"; echo
    exit 1
}

echo; echo "Building Docker image from $1"

OPERATOR_DOCKER_BUILD_NAME="$(docker_generateBuildName)"
branch="$(pipeline_gitBranchName)"
dockerVirtualRepository="$(docker_devArtifactRepoName ${branch})"

UBI_MICRO_VERSION="9.4.15"
UBI_MINIMAL_VERSION="9.4.1227"

echo "Building and pushing a multiarch image through  buildx"
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes

# BuildKit v0.32.x bundles runc v1.4.3, which masks directories with a tmpfs
# mounted as nr_inodes=1. Kernels that reject that value fail every RUN step
# with 'can't mask dir "/proc/acpi"' (opencontainers/runc#5348). Unpin once a
# BuildKit release ships runc >= 1.4.4.
BUILDKIT_IMAGE="${BUILDKIT_IMAGE:-docker.jfrog.io/moby/buildkit:v0.30.0}"
docker buildx rm mybuilder > /dev/null 2>&1 || true
docker buildx create --name mybuilder --driver docker-container \
       --driver-opt image="${BUILDKIT_IMAGE}" --use \
        || errorExit "Failed creating buildx builder mybuilder"
docker buildx build --no-cache --platform linux/arm64,linux/amd64 --build-arg UBI_MICRO=${UBI_MICRO_VERSION} --build-arg UBI_MINIMAL=${UBI_MINIMAL_VERSION} \
       -t ${2} -f "${3}/Dockerfile" --metadata-file=build-metadata-${1}  --push . \
        || errorExit "Failed building ${1} docker image"

echo "Publishing build info for multi-arch image"
echo "Running the following command: jf rt build-docker-create ${dockerVirtualRepository} --server-id=entplus_deployer --image-file build-metadata-${1} --build-name ${OPERATOR_DOCKER_BUILD_NAME} --build-number ${JFROG_CLI_BUILD_NUMBER}"
cat build-metadata-${1}
jf rt build-docker-create ${dockerVirtualRepository} --server-id=entplus_deployer --image-file build-metadata-${1} --build-name ${OPERATOR_DOCKER_BUILD_NAME} --build-number ${JFROG_CLI_BUILD_NUMBER}

jf rt build-publish --server-id=entplus_deployer  ${OPERATOR_DOCKER_BUILD_NAME} ${JFROG_CLI_BUILD_NUMBER}