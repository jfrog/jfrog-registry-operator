#!/usr/bin/env bash

# https://codeinthehole.com/tips/bash-error-reporting/
set -eu -o pipefail
function print_error {
    read line file <<<$(caller)
    echo "An error occurred in line $line of file $file:" >&2
    sed "${line}q;d" "$file" >&2
}
trap print_error ERR

DEBUG=${DEBUG:=false}

PROJECT_DIR="${PROJECT_DIR:-}"
if [[ -z "${PROJECT_DIR}" ]]; then
  echo "ERROR: PROJECT_DIR is not defined."
  exit 1
fi

GOCMD=${GOCMD:-$(which go || echo '')}
if [[ -z "${GOCMD}" ]]; then
  echo "ERROR: Go command is not defined."
  exit 1
fi

GOOS=${GOOS:-$(go env GOOS)}
GOARCH=${GOARCH:-$(go env GOARCH)}
BINARYNAME=${BINARYNAME:-$(go env BINARYNAME)}

FILENAME="${BINARYNAME}-${GOARCH}"
BIN_DIR="${BIN_DIR:-${PROJECT_DIR}/bin}"
OUTPUT_FILE="${BIN_DIR}/${FILENAME}"
CMD_SRC_DIR=${CMD_SRC_DIR:-$(go env CMD_SRC_DIR)}

function doBuild {
  if [[ ${DEBUG} == true ]]; then
    set -x
  fi
  ${GOCMD} build -o ${OUTPUT_FILE} ${CMD_SRC_DIR}/main.go
  set +x
}

echo "Building ${FILENAME} ..."

if [[ ${DEBUG} == true ]]; then
  GO_VERSION="$(${GOCMD} version)"
  echo "Using Go: ${GO_VERSION}"
  echo "  Command (GOCMD):   ${GOCMD}"
  echo "  OS      (GOOS):    ${GOOS}"
  echo "  Arch    (GOARCH):  ${GOARCH}"
  echo "Project:"
  echo "  Root:     ${PROJECT_DIR}"
  echo "  Output:   ${BIN_DIR}"
  echo "  Filename: ${FILENAME}"

  time doBuild

  echo ""
  echo "Output file:"
  ls -l -h "${OUTPUT_FILE}"
  echo ""
else
  doBuild
fi

echo "Done."