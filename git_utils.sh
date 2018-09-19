#!/bin/bash
# Copyright 2014 The Kubernetes Authors.
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

#Storage location from local git config
git_object_storage () {
    git_lfs_storage=`git config --local --get lfs.storage`
    if [ ! -z "$git_lfs_storage" ]; then
        git_lfs_base_dir=$(dirname $git_lfs_storage)
    fi
    echo $git_lfs_base_dir
}

#Get volume mount for container-build
container_git_storage_mount () {
    object_store=$(git_object_storage)
    if [ ! -z "$object_store" ]; then
        echo "-v $object_store:$object_store"
    fi
}
