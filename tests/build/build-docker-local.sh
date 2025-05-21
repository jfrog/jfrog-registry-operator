#!/bin/bash

export PROJECT_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"

errorExit () {
    echo; echo "ERROR: $1"; echo
    exit 1
}

echo; echo "Building Docker image from $1"

UBI_MICRO_VERSION=9.4.15
UBI_MINIMAL_VERSION=9.4.1227
echo $pwd
echo "Building and pushing a image"
# docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
# docker buildx create --name mybuilder --use || true
docker buildx build --no-cache --platform ${4} --build-arg UBI_MICRO=${UBI_MICRO_VERSION} --build-arg UBI_MINIMAL=${UBI_MINIMAL_VERSION} \
       -t ${2} -f "${3}/Dockerfile" --load . \
        || errorExit "Failed building ${1} docker image"