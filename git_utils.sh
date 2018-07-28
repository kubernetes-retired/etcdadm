#!/bin/bash

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
