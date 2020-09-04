# etcd-manager

etcd-manager manages an etcd cluster, on a cloud or on bare-metal.

It borrows from the ideas of the etcd-operator, but avoids the circular dependency of relying on kubernetes (which itself relies on etcd).

etcd-manager performs a minimal amount of gossip-like discovery of peers for initial cluster membership, then "pivots to etcd" as soon as possible.
etcd-manager also watches for peer membership changes to manage the correct number of members for the desired cluster quorum.
Actual cluster membership changes to etcd continue to take place through raft.

Communication with etcd-manager happens via the `etcd-manager-ctl` binary.

A walkthrough of the process of setting up a cluster for local development can be found [here](docs/walkthrough.md).

## Backups
The etcd-manager leader periodically backs up etcd snapshots to a backup store, in addition to automatically triggering an backup before cluster-modification operations.

Cluster changes that are not normally supported can be done safely through etcd backups & restore.

The format of the backup store, which is intended to be project-neutral is [here](docs/backupstructure.md).

## Code overview

### etcd-host

* Each node can host
* The state is stored in a local directory; typically this would be a persistent volume
* etcd-manager runs the local etcd process if configured/told to start by the etcd-manager leader, restarts it after a crash, etc.
* Code is in `pkg/etcd`

### etcd-manager-ctl
* Communicates with the etcd-manager leader via a S3 work queue

Supported Commands:
```
get				Shows Cluster Spec
configure-cluster		Sets cluster spec based on -member-count and -etcd-version args specified.
list-backups			List backups available in the -backup-store
list-commands			List commands in queue for cluster to execute.
delete-command			Deletes a command from the clusters queue
restore-backup			Restores the backup specified. Pass the backup timestamp shown by list-backup as parameter.
				eg. etcd-ctl -backup-store=s3://mybackupstore/ restore-backup 2019-05-07T18:28:01Z-000977
```

### etcd-manager
* Runs the leader election and control loop
* Hosts a gRPC server to run gossip discovery of peers and communicate initial setup parameters

#### Leader Election Overview

* Each node that can be a master starts a controller loop.
* Each controller gossips with other controllers that it discovers (initially through seeding, then through gossip)
* Seed discovery is pluggable
* A controller may (or may not) be running an etcd process
* The controllers try to elect a "weak leader", by choosing the member with the lowest ID.
* A controller that believes itself to be the leader sends a message to every peer, and any peer can reject the leadership bid (if it knows of a peer with a lower ID)
* Upon leader election, the controller may try to take a shared lock (for additional insurance against multiple leaders)
* Gossip and leader election code is in `pkg/privateapi`

#### Leader Control Loop

Once a leader has been determined, it performs this basic loop:

* It queries every peer to see if it is hosting an etcd process
* If no peer is running etcd, it will try to start etcd.  The leader picks an initial membership set (satisfying quorum) and requests those peers to start the cluster.  (The leader may choose itself)
* If there is an etcd cluster but it has fewer members than desired, the leader will try to pick a node to add to the cluster; it communicates with the peer to start the cluster.
* If there is an etcd cluster but it has more members than desired, the leader will try to remove an etcd cluster member.
* If there is an etcd cluster but it has unhealthy members, the leader will try to remove an unhealthy etcd cluster member.
* Commands from the S3 work queue are performed.
* Periodically a backup of etcd is performed.

## Current shortcomings

Help gratefully received:

* ~We need to split out the backup logic, so that we can run it as a simple coprocess
  alongside existing etcd implementations.~
* We should better integrate settting the spec into kubernetes.  A controller could sync it
  with a CRD or apimachinery type.
* We use the VFS library from kops (that is the only dependency on kops, and it's not a big one).  We should look at making VFS
  into a true kubernetes shared library.
* ~We should probably not recover automatically from a backup in the event of total cluster loss, because backups are periodic
  and thus we know some data loss is likely.  Idea: drop a marker file into the backup store.~
* The controller leader election currently considers itself the leader when it has consensus amongst all reachable peers,
  and will create a cluster when there are sufficient peers to form a quorum. But with partitions, it's possible to have
  two nodes that both believe themselves to be the leader.  If the number of machines is `>= 2 * quorum` then we could
  form two etcd clusters (etcd itself should stop overlapping clusters).  A pluggable locking implementation is one
  solution in progress; GCS has good consistency guarantees.
* ~Discovery mechanisms are currently mostly fake - they work on a local filesystem.  We have an early one backed by VFS,
  but discovery via the EC2/GCE APIs would be great, as would network scanning or multicast discovery.~
* All cluster version changes currently are performed via the "full dump and restore" mechanism.  We should learn
  that some version changes are in fact safe, and perform them as a rolling-update (after a backup!)
* ~There should be a way to trigger a backup via a convenient mechanism.  Idea: drop a marker key into etcd.~
