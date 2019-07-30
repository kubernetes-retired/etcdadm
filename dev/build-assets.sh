#!/bin/bash

VERSION=$1
PLATFORMS="linux_amd64 darwin_amd64 windows_amd64"
CMDS="etcd-manager-ctl"

# Ensure the dist folder exists and is clean
rm -fr dist/${VERSION} && mkdir -p dist/${VERSION}

for CMD in ${CMDS}; do
    for PLATFORM in ${PLATFORMS}; do
        CMD_DIST_PATH="dist/${VERSION}/${CMD}-${PLATFORM/_/-}"

        # Get the expected binary file extension
        if [[ "${PLATFORM}" =~ "windows" ]]; then
            EXTENSION=".exe"
        else
            EXTENSION=""
        fi

        # Build
        bazel build --platforms=@io_bazel_rules_go//go/toolchain:${PLATFORM} //cmd/${CMD}

        if [ -e "bazel-bin/cmd/${CMD}/${PLATFORM}_stripped/${CMD}${EXTENSION}" ]; then
            cp bazel-bin/cmd/${CMD}/${PLATFORM}_stripped/${CMD}${EXTENSION} ${CMD_DIST_PATH}
        elif [ -e "bazel-bin/cmd/${CMD}/${PLATFORM}_pure_stripped/${CMD}${EXTENSION}" ]; then
            cp bazel-bin/cmd/${CMD}/${PLATFORM}_pure_stripped/${CMD}${EXTENSION} ${CMD_DIST_PATH}
        else
            echo "Unable to find compiled binary for ${CMD} ${PLATFORM}"
            exit 1
        fi

        # Generate SHA-256
        shasum -a 256 ${CMD_DIST_PATH} | cut -d ' ' -f 1 > ${CMD_DIST_PATH}-sha-256
    done
done
