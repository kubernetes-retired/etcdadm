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

# set REPO_PATH
REPO_PATH=$(get_root_path)
cd "${REPO_PATH}"

# failed tests, if a script fails we'll add to this arry
failures=()

# run all verify scripts, optionally skipping any of them

if [[ "${VERIFY_WHITESPACE:-true}" == "true" ]]; then
  echo "[*] Verifying whitespace..."
  hack/verify-whitespace.sh || failures+=("hack/verify-whitespace.sh")
  cd "${REPO_PATH}"
fi

if [[ "${VERIFY_SPELLING:-true}" == "true" ]]; then
  echo "[*] Verifying spelling..."
  hack/verify-spelling.sh || failures+=("hack/verify-spelling.sh")
  cd "${REPO_PATH}"
fi

if [[ "${VERIFY_BOILERPLATE:-true}" == "true" ]]; then
  echo "[*] Verifying boilerplate..."
  hack/verify-boilerplate.sh || failures+=("hack/verify-boilerplate.sh")
  cd "${REPO_PATH}"
fi

if [[ "${VERIFY_GOFMT:-true}" == "true" ]]; then
  echo "[*] Verifying gofmt..."
  hack/verify-gofmt.sh || failures+=("hack/verify-gofmt.sh")
  cd "${REPO_PATH}"
fi

if [[ "${VERIFY_GOLINT:-true}" == "true" ]]; then
  echo "[*] Verifying golint..."
  hack/verify-golint.sh || failures+=("hack/verify-golint.sh")
  cd "${REPO_PATH}"
fi

if [[ "${VERIFY_GOVET:-true}" == "true" ]]; then
  echo "[*] Verifying govet..."
  hack/verify-govet.sh || failures+=("hack/verify-govet.sh")
  cd "${REPO_PATH}"
fi

if [[ "${VERIFY_DEPS:-true}" == "true" ]]; then
  echo "[*] Verifying deps..."
  hack/verify-deps.sh || failures+=("hack/verify-deps.sh")
  cd "${REPO_PATH}"
fi

if [[ "${VERIFY_GOTEST:-true}" == "true" ]]; then
  echo "[*] Verifying gotest..."
  hack/verify-gotest.sh || failures+=("hack/verify-gotest.sh")
  cd "${REPO_PATH}"
fi

if [[ "${VERIFY_BUILD:-true}" == "true" ]]; then
  echo "[*] Verifying build..."
  hack/verify-build.sh || failures+=("hack/verify-build.sh")
  cd "${REPO_PATH}"
fi

if [[ "${VERIFY_VERSION:-true}" == "true" ]]; then
  echo "[*] Verifying version..."
  hack/verify-version.sh || failures+=("hack/verify-version.sh")
  cd "${REPO_PATH}"
fi

# exit based on verify scripts
if [[ "${#failures[@]}" -eq 0 ]]; then
  echo ""
  echo "All verify checks passed, congrats!"
else
  echo ""
  echo "One or more verify checks failed! Full output is above.  Failed:"
  for failure in "${failures}"; do
    echo "  ${failure}"
  done
  exit 1
fi
