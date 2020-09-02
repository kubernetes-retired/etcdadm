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

# run go test
export GO111MODULE=on
ERROR=0
# To ease merging repos, we initially exclude etcd-manager, then will fix it here
DIRS=$(git ls-files | grep -v "vendor\/" | grep -v "etcd-manager\/" | grep ".go" | xargs dirname | grep -v "\." | cut -d '/' -f 1 | uniq)
while read -r dir; do
    go test ./"$dir"/... || ERROR=1
done <<< "$DIRS"
go test *.go || ERROR=1

if [[ "${ERROR}" = 1 ]]; then
    echo "found errors in go test!"
fi

exit ${ERROR}
