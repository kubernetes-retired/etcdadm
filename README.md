# etcd-manager

etcd-manager manages an etcd cluster, on a cloud or on bare-metal.

It borrows from the ideas of the etcd-operator, but avoids the circular dependency of relying on kubernetes (which itself relies on etcd).

The key insight is that we aim to "pivot to etcd" as soon as possible.  So while we use a gossip-like mechanism to discover the initial cluster membership and to elect a coordinator, we then have etcd start a cluster.  Cluster membership changes take place through raft.

We maintain a backup store, to which we back up etcd periodically.  Backups are automatically triggered before cluster-modification operations.

Cluster changes that are not normally supported can be done safely through etcd backups & restore.

The format of the backup store, which is intended to be project-neutral is [here](docs/backupstructure.md).

## Walkthough

We'll now do a walkthrough to get going with local development.  In production you likely would run this using
a packaged docker container, but this walkthrough lets you see what's going on here.

etcd must be installed in `/opt/etcd-v2.2.1-linux-amd64/etcd`, etcdctl in `/opt/etcd-v2.2.1-linux-amd64/etcdctl`.  Each version of etcd you want to run must be installed in the same pattern.  Make sure you've downloaded `/opt/etcd-v3.2.18-linux-amd64` for this demo. (Typically you'll run etcd-manager in a docker image)

NOTE: If you're running on OSX, CoreOS does not ship a version of etcd2 that runs correctly on recent versions os OSX.  Running inside Docker avoids this problem.

```
# Clean up from previous runs
rm -rf /tmp/discovery/*
rm -rf /tmp/etcd-manager/*

# Build code
bazel build //cmd/etcd-manager //cmd/etcd-manager-ctl
ln -sf bazel-bin/cmd/etcd-manager-ctl/linux_amd64_stripped/etcd-manager-ctl
ln -sf bazel-bin/cmd/etcd-manager/linux_amd64_stripped/etcd-manager

# Start etcd manager
./etcd-manager --address 127.0.0.1 --cluster-name=test --backup-store=file:///tmp/etcd-manager/backups/test --data-dir=/tmp/etcd-manager/data/test/1 --client-urls=http://127.0.0.1:4001 --quarantine-client-urls=http://127.0.0.1:8001

# Seed cluster creation
./etcd-manager-ctl --members=1 --backup-store=file:///tmp/etcd-manager/backups/test --etcd-version=2.2.1
```

`etcd-manager` will start a node ready to start running etcd, and `etcd-manager-ctl` will provide the initial settings
for the cluster.  Those settings are written to the backup store, so the backup store acts as a source of truth when
etcd is not running.  So this will start a single node cluster of etcd (`--members=1`),
running etcd version 2.2.1 (`--etcd-version=2.2.1`).

You should be able to set and list keys using the etcdctl tool:

```
> curl -XPUT -d "value=world"  http://127.0.0.1:4001/v2/keys/hello
{"action":"set","node":{"key":"/hello","value":"world","modifiedIndex":6,"createdIndex":6}}
> curl http://127.0.0.1:4001/v2/keys/hello
{"action":"get","node":{"key":"/hello","value":"world","modifiedIndex":6,"createdIndex":6}}
```

Now if we want to expand the cluster (it's probably easiest to run each of these commands in different windows / tabs / tmux windows / screen windows):

```
./etcd-manager --address 127.0.0.2 --cluster-name=test --backup-store=file:///tmp/etcd-manager/backups/test --data-dir=/tmp/etcd-manager/data/test/2 --client-urls=http://127.0.0.2:4001 --quarantine-client-urls=http://127.0.0.2:8001
./etcd-manager --address 127.0.0.3 --cluster-name=test --backup-store=file:///tmp/etcd-manager/backups/test --data-dir=/tmp/etcd-manager/data/test/3 --client-urls=http://127.0.0.3:4001 --quarantine-client-urls=http://127.0.0.3:8001
```

Within a few seconds, the two other nodes will join the gossip cluster, but will not yet be part of etcd.  The leader controller will be logging something like this:

```
etcd cluster state: etcdClusterState
  members:
    {"name":"GEyIUF0s9J07setaePSQ3Q","peerURLs":["http://127.0.0.1:2380"],"clientURLs":["http://127.0.0.1:4001"],"ID":"4be33a4562894143"}
  peers:
    etcdClusterPeerInfo{peer=peer{id:"yFQlUrhyV2P0i14oXydBbw" addresses:"127.0.0.3:8000" }, info=cluster_name:"test" node_configuration:<name:"yFQlUrhyV2P0i14oXydBbw" peer_urls:"http://127.0.0.3:2380" client_urls:"http://127.0.0.3:4001" quarantined_client_urls:"http://127.0.0.3:8001" > }
    etcdClusterPeerInfo{peer=peer{id:"GEyIUF0s9J07setaePSQ3Q" addresses:"127.0.0.1:8000" }, info=cluster_name:"test" node_configuration:<name:"GEyIUF0s9J07setaePSQ3Q" peer_urls:"http://127.0.0.1:2380" client_urls:"http://127.0.0.1:4001" quarantined_client_urls:"http://127.0.0.1:8001" > etcd_state:<cluster:<cluster_token:"XypoARu3zvWjVHHmWSGr_Q" nodes:<name:"GEyIUF0s9J07setaePSQ3Q" peer_urls:"http://127.0.0.1:2380" client_urls:"http://127.0.0.1:4001" quarantined_client_urls:"http://127.0.0.1:8001" > > etcd_version:"2.2.1" > }
    etcdClusterPeerInfo{peer=peer{id:"mW4JV4NzL-hO39BEU8gnhA" addresses:"127.0.0.2:8000" }, info=cluster_name:"test" node_configuration:<name:"mW4JV4NzL-hO39BEU8gnhA" peer_urls:"http://127.0.0.2:2380" client_urls:"http://127.0.0.2:4001" quarantined_client_urls:"http://127.0.0.2:8001" > }
```

Maintaining this state is essentially the core controller loop.  The leader pings each peer,
asking if it is configured to run etcd, and also queries the state of etcd and the desired spec.
Where there are differences, the controller attempts to converge the state by adding/removing members,
doing backups/restores and changing versions.


If you do look around the directories:

* `ls /tmp/discovery` will show how the VFS-backed discovery mechanism works
* `ls -R /tmp/etcd-manager/backups/` shows the structure of the backup store
* `ls -R /tmp/etcd-manager/data` has the backup itself


We can reconfigure the cluster:

```
> curl http://127.0.0.1:4001/v2/keys/kope.io/etcd-manager/test/spec
{"action":"get","node":{"key":"/kope.io/etcd-manager/test/spec","value":"{\n  \"memberCount\": 1,\n  \"etcdVersion\": \"2.2.1\"\n}","modifiedIndex":4,"createdIndex":4}}

> curl -XPUT -d 'value={ "memberCount": 3, "etcdVersion": "2.2.1" }' http://127.0.0.1:4001/v2/keys/kope.io/etcd-manager/test/spec
```

Within a minute, we should see all 3 nodes in the etcd cluster:

```
> curl http://127.0.0.1:4001/v2/members/
{"members":[{"id":"c05691f8951bfaf5","name":"FD2NN12mS_K4ovji8dTL9g","peerURLs":["http://127.0.0.1:2380"],"clientURLs":["http://127.0.0.1:4001"]},{"id":"c3dd045ca41acb5f","name":"ykjVfcnf5DWUCwpKSBQFEg","peerURLs":["http://127.0.0.3:2380"],"clientURLs":["http://127.0.0.3:4001"]},{"id":"d0c3c4397fa244d1","name":"d7qDvoAwX5bhhUwQo0CHgg","peerURLs":["http://127.0.0.2:2380"],"clientURLs":["http://127.0.0.2:4001"]}]}
```

Play around with stopping & starting the nodes, or with removing the data for one at a time.  The cluster should be available whenever a majority of the processes are running,
and as long as you allow the cluster to recover before deleting a second data directory, no data should be lost.
(We'll cover "majority" disaster recovery later)

### Disaster recovery

The etcd-manager performs periodic backups.  In the event of a total failure, it will restore automatically
(TODO: we should make this configurable - if a node _could_ recover we likely want this to be manually triggered)

Verify backup/restore works correctly:

```
curl -XPUT -d "value=world"  http://127.0.0.1:4001/v2/keys/hello
```

* Wait for a "took backup" message (TODO: should we be able to force a backup?)
* Stop all 3 processes
* Remove the active data: `rm -rf /tmp/etcd-manager/data/test`
* Restart all 3 processes

Disaster recovery will detect that no etcd nodes are running, will start a cluster on all 3 nodes, and restore the backup.


### Upgrading

```
> curl http://127.0.0.1:4001/v2/keys/kope.io/etcd-manager/test/spec
{"action":"get","node":{"key":"/kope.io/etcd-manager/test/spec","value":"{ \"memberCount\": 3, \"etcdVersion\": \"2.2.1\" }","modifiedIndex":8,"createdIndex":8}}

> curl -XPUT -d 'value={ "memberCount": 3, "etcdVersion": "3.2.18" }' http://127.0.0.1:4001/v2/keys/kope.io/etcd-manager/test/spec
```

Dump keys to be sure that everything copied across:
```
> ETCDCTL_API=3 /opt/etcd-v3.2.18-linux-amd64/etcdctl --endpoints http://127.0.0.1:4001 get "" --prefix
/hello
world
/kope.io/etcd-manager/test/spec
{ "memberCount": 3, "etcdVersion": "3.2.18" }
```

You may note that we did the impossible here - we went straight from etcd 2 to etcd 3 in an HA cluster.  There was some
downtime during the migration, but we performed a logical backup:

* Quarantined each etcd cluster members, so that they cannot be reached by clients.  This prevents the etcd data from changing.
* Performs a backup (using `etcdctl backup ...`)
* Stops all the etcd members in the cluster
* Creates a new cluster running the new version, quarantined
* Restores the backup into the new cluster
* Lifts the quarantine on the new cluster, so that it is reachable

This procedure works between any two versions of etcd (or even with other storage mechanisms).  It has the downside
that the etcd-version of each object is changed, meaning all watches are invalidated.

TODO: We should enable "hot" upgrades where the version change is compatible.  (It's easy, but it's nice to have one code path for now)

If you want to try a downgrade:
`ETCDCTL_API=3 /opt/etcd-v3.2.18-linux-amd64/etcdctl --endpoints http://127.0.0.1:4001  put /kope.io/etcd-manager/test/spec '{ "memberCount": 3, "etcdVersion": "2.3.7" }'`

## Code overview

### etcd-host

* Each node can host etcd
* The state is stored in a local directory; typically this would be a persistent volume
* etcd-manager runs the local etcd process if configured, restarts it after a crash, etc.
* Code is in `pkg/etcd`

### Leader Election Overview

* Each node that can be a master starts a controller loop.
* Each controller gossips with other controllers that it discovers (initially through seeding, then through gossip)
* Seed discovery is pluggable
* A controller may (or may not) be running an etcd process
* The controllers try to elect a "weak leader", by choosing the member with the lowest ID.
* A controller that believes itself to be the leader sends a message to every peer, and any peer can reject the leadership bid (if it knows of a peer with a lower ID)
* Upon leader election, the controller may try to take a shared lock (for additional insurance against multiple leaders)
* Gossip and leader election code is in `pkg/privateapi`

### Leader Control Loop

Once a leader has been determined, it performs this basic loop:

* It queries every peer to see if it is hosting an etcd process
* If no peer is running etcd, it will try to start etcd.  The leader picks an initial membership set (satisfying quorum) and requests those peers to start the cluster.  (The leader may choose itself)
* If there is an etcd cluster but it has fewer members than desired, the leader will try to pick a node to add to the cluster; it communicates with the peer to start the cluster.
* If there is an etcd cluster but it has more members than desired, the leader will try to remove an etcd cluster member.
* If there is an etcd cluster but it has unhealthy members, the leader will try to remove an unhealthy etcd cluster member.
* Periodically a backup of etcd is performed.

## Current shortcomings

Help gratefully received:

* We need to split out the backup logic, so that we can run it as a simple coprocess
  alongside existing etcd implementations.
* We should better integrate settting the `/kope.io/etcd-manager/test/spec` into kubernetes.  A controller could sync it
  with a CRD or apimachinery type.
* We use the VFS library from kops (that is the only dependency on kops, and it's not a big one).  We should look at making VFS
  into a true kubernetes shared library.
* We should probably not recover automatically from a backup in the event of total cluster loss, because backups are periodic
  and thus we know some data loss is likely.  Idea: drop a marker file into the backup store.
* The controller leader election currently considers itself the leader when it has consensus amongst all reachable peers,
  and will create a cluster when there are sufficient peers to form a quorum. But with partitions, it's possible to have
  two nodes that both believe themselves to be the leader.  If the number of machines is `>= 2 * quorum` then we could
  form two etcd clusters (etcd itself should stop overlapping clusters).  A pluggable locking implementation is one
  solution in progress; GCS has good consistency guarantees.
* Discovery mechanisms are currently mostly fake - they work on a local filesystem.  We have an early one backed by VFS,
  but discovery via the EC2/GCE APIs would be great, as would network scanning or multicast discovery.
* All cluster version changes currently are performed via the "full dump and restore" mechanism.  We should learn
  that some version changes are in fact safe, and perform them as a rolling-update (after a backup!)
* There should be a way to trigger a backup via a convenient mechanism.  Idea: drop a marker key into etcd.
