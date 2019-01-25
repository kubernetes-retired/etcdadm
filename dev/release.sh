#!/bin/bash

TODAY=`date +%Y%m%d`
echo "# Run these commands to do a release"
echo "DOCKER_IMAGE_PREFIX=kopeio/ DOCKER_TAG=3.0.${TODAY} make push"
echo "DOCKER_IMAGE_PREFIX=kopeio/ DOCKER_TAG=latest make push"
