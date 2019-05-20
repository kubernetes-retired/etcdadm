#!/bin/bash
set -x
NAME=$(basename $(git remote get-url origin | sed 's/\.git//'))
GITHUB_USER=$(basename $(dirname $(git remote get-url origin | sed 's/\.git//')))
TAG=$(git tag --points-at HEAD )

GO111MODULE=off go get github.com/aktau/github-release
which upx > /dev/null  || (sudo apt-get update && sudo apt-get install -y upx-ucl)
make clean $NAME
upx $NAME
github-release release --user $GITHUB_USER --repo $NAME --tag $TAG
github-release upload --user $GITHUB_USER --repo $NAME --tag $TAG --name $NAME --file $NAME
