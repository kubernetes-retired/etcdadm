#!/bin/bash

TODAY=`date +%Y%m%d`
echo "# Run this command to do a release"
echo "DOCKER_IMAGE_PREFIX=kopeio/ DOCKER_TAG=1.0.${TODAY} make push"
