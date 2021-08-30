# Release Process

etcdadm & etcd-manager are released on an as-needed basis.

## Check builds OK

Ensure the commit you are tagging is showing as green in github & prow test results.

## Tag the branch

(TODO: Automate this following the kOps pull-request pattern)

```
git checkout master
git fetch origin
git reset --hard origin/master
```

Make sure you are on the correct commit, and not a newer one!

```
TODAY=`date +%Y%m%d`
VERSION="3.0.${TODAY}"
echo "VERSION=${VERSION}"
```

We push an annotated tag:
```
git tag -a etcd-manager/v${VERSION} -m "etcd-manager/v${VERSION}"
git show etcd-manager/v${VERSION}
```

Double check it is the correct commit!

```
git push git@github.com:kubernetes-sigs/etcdadm etcd-manager/v${VERSION}
git fetch origin # sync back up
```


## Wait for staging job to complete

The staging job should now see the tag, and build it (from the trusted prow cluster, using Google Cloud Build).

The job is here: https://testgrid.k8s.io/sig-cluster-lifecycle-etcdadm#etcdadm-postsubmit-push-to-staging

It (currently) takes about 10 minutes to run.

In the meantime, you can compile the release notes...

## Compile release notes

e.g.

```
git checkout -b relnotes_${VERSION}

PREVIOUS_TAG=`git tag -l | grep etcd-manager/v | tail -n2 | head -n -1 | sed -e s@etcd-manager/v@@g`
LATEST_TAG=`git tag -l | grep etcd-manager/v | tail -n1 | sed -e s@etcd-manager/v@@g`
git log etcd-manager/v${PREVIOUS_TAG}..etcd-manager/v${LATEST_TAG} --oneline | grep Merge.pull | grep -v Revert..Merge.pull | cut -f 5 -d ' ' | tac  > /tmp/prs
echo -e "\n# ${LATEST_TAG}\n"  >> etcd-manager/docs/releases/3.0.md
relnotes  -config etcd-manager/.shipbot.yaml  < /tmp/prs  >> etcd-manager/docs/releases/3.0.md
```

Review then send a PR with the release notes:

```
git add -p etcd-manager/docs/releases/3.0.md
git commit -m "Release notes for etcd-manager ${VERSION}"
gh pr create --fill
```

## Propose promotion of artifacts

Create container promotion PR:

```
STAGING_VERSION=v${VERSION}
RELEASE_VERSION=v${VERSION}

# Should show image
gcrane ls gcr.io/k8s-staging-etcdadm/etcd-manager | grep "gcr.io/k8s-staging-etcdadm/etcd-manager:${STAGING_VERSION}"
```

```
cd ${GOPATH}/src/k8s.io/k8s.io

git checkout main
git pull
git checkout -b etcdadm_images_${RELEASE_VERSION}

cd k8s.gcr.io/images/k8s-staging-etcdadm
echo "" >> images.yaml
echo "# ${RELEASE_VERSION}" >> images.yaml
k8s-container-image-promoter --snapshot gcr.io/k8s-staging-etcdadm --snapshot-tag ${STAGING_VERSION} | sed s@${STAGING_VERSION}@${RELEASE_VERSION}@g >> images.yaml
```

You can dry-run the promotion with

```
cd ${GOPATH}/src/k8s.io/k8s.io
k8s-container-image-promoter --thin-manifest-dir k8s.gcr.io
```

Send the image promotion PR:

```
cd ${GOPATH}/src/k8s.io/k8s.io
git add -p k8s.gcr.io/images/k8s-staging-etcdadm/images.yaml
git commit -m "Promote etcdadm $RELEASE_VERSION images"
gh pr create --base main --fill
```


## Smoketesting the release

More process coming soon, but in the meantime override the version
in a kOps cluster and validate.  Send the PR to kOps development branch and
let it go through e2e-tests before cherry-picking it.

## On github

* Add release notes
* Publish it
