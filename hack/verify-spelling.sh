#!/usr/bin/env bash
# Copyright 2019 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

# shellcheck source=/dev/null
source "$(dirname "$0")/utils.sh"
# cd to the root path
cd_root_path

# create a temporary directory
TMP_DIR=$(mktemp -d)

# cleanup
exitHandler() (
  echo "Cleaning up..."
  rm -rf "${TMP_DIR}"
)
trap exitHandler EXIT


# build misspell
pushd "hack/tools" >/dev/null
MISSPELL_CMD="${TMP_DIR}/misspell"
GO111MODULE=on go build -o ${MISSPELL_CMD} github.com/client9/misspell/cmd/misspell
popd

# check spelling
RES=0
ERROR_LOG="${TMP_DIR}/errors.log"
echo "Checking spelling..."
git ls-files | grep -v -e vendor | xargs "${MISSPELL_CMD}" > "${ERROR_LOG}"
if [[ -s "${ERROR_LOG}" ]]; then
  sed 's/^/error: /' "${ERROR_LOG}" # add 'error' to each line to highlight in e2e status
  echo "Found spelling errors!"
  RES=1
else
  echo "No spelling errors found"
fi
exit "${RES}"
