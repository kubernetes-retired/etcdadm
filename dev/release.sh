#!/bin/bash

TODAY=`date +%Y%m%d`
VERSION="3.0.${TODAY}"

echo "# Run these commands to do a release"
echo "DOCKER_IMAGE_PREFIX=kopeio/ DOCKER_TAG=${VERSION} make push"
echo "DOCKER_IMAGE_PREFIX=kopeio/ DOCKER_TAG=latest make push"
echo "git tag ${VERSION}"
echo "git push --tags ssh://git@github.com/kopeio/etcd-manager"
echo "./dev/build-assets.sh ${VERSION}"
echo "# Finally, create a new release on GitHub attaching binary assets at dist/"
echo "shipbot -config .shipbot.yaml -tag ${VERSION}"
echo "# To compile release notes:"
echo "git log --oneline <previous>..${VERSION} | grep Merge.pull | cut -f 5 -d ' ' | tac  > /tmp/prs"
echo "echo '# ${VERSION}' >> docs/releases/3.0.md"
echo "relnotes  -config .shipbot.yaml  < /tmp/prs >> docs/releases/3.0.md"
