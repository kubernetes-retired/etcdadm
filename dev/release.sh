#!/bin/bash

TODAY=`date +%Y%m%d`
echo "# Run these commands to do a release"
echo "DOCKER_IMAGE_PREFIX=kopeio/ DOCKER_TAG=3.0.${TODAY} make push"
echo "DOCKER_IMAGE_PREFIX=kopeio/ DOCKER_TAG=latest make push"
echo "git tag 3.0.${TODAY}"
echo "git push --tags ssh://git@github.com/kopeio/etcd-manager"
