# etcdadm Use Cases

As a Kubernetes cluster administrator:

## Fault-Tolerance

1. I want to deploy an etcd cluster that can tolerate node failures.

## Upgrades

0. When I upgrade Kubernetes from vA.B.C to vX.Y.Z, I want to upgrade the etcd cluster to the version recommended for Kubernetes vX.Y.Z.

0. I want to deploy new versions of etcd quickly when patch releases (e.g. for CVEs) are released.

## Performance

0. I want to ensure that etcd members have priority access to disk.

0. I want to ensure that etcd members have a guaranteed amount of memory.

## Disaster Recovery

0. I want to (continuously) backup etcd state to external storaqe.

0. I want to create an etcd cluster from state backed-up to external storage.

## Diverse Environments

0. I want to use etcdadm in an offline environment.

