#!/bin/bash
# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

VERSION=$1
PLATFORMS="linux_amd64 darwin_amd64 windows_amd64"
CMDS="etcd-manager-ctl"

# cd to the etcd-manager root
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "${REPO_ROOT}"/etcd-manager

# Ensure the dist folder exists and is clean
rm -fr dist/ && mkdir -p dist/

for CMD in ${CMDS}; do
    for PLATFORM in ${PLATFORMS}; do
        CMD_DIST_PATH="dist/${CMD}-${PLATFORM/_/-}"

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
