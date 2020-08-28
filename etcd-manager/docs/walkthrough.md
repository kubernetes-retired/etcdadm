## Walkthrough

We'll now do a walkthrough to get going with local development.  In production you likely would run this using
a packaged docker container, but this walkthrough lets you see what's going on here.

etcd must be installed in `/opt/etcd-v2.2.1-linux-amd64/etcd`, etcdctl in `/opt/etcd-v2.2.1-linux-amd64/etcdctl`.  Each version of etcd you want to run must be installed in the same pattern.  Make sure you've downloaded `/opt/etcd-v3.2.18-linux-amd64` for this demo. (Typically you'll run etcd-manager in a docker image)

On Linux, you can do this with:

```
bazel build //:etcd-v2.2.1-linux-amd64_etcd //:etcd-v2.2.1-linux-amd64_etcdctl
bazel build //:etcd-v3.2.24-linux-amd64_etcd //:etcd-v3.2.24-linux-amd64_etcdctl
sudo cp -r bazel-genfiles/etcd-v* /opt/
sudo chown -R ${USER} /opt/etcd-v*
```

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
./etcd-manager --insecure --etcd-insecure --address 127.0.0.1 --etcd-address 127.0.0.1 --cluster-name=test --backup-store=file:///tmp/etcd-manager/backups/test --data-dir=/tmp/etcd-manager/data/test/1 --client-urls=http://127.0.0.1:4001 --quarantine-client-urls=http://127.0.0.1:8001 --peer-urls=http://127.0.0.1:2380

# Seed cluster creation
./etcd-manager-ctl -member-count=1 --backup-store=file:///tmp/etcd-manager/backups/test -etcd-version=2.2.1 configure-cluster
```

Note the `--insecure` and `--etcd-insecure` flags - we're turning off TLS for
both etcd-manager and etcd - you shouldn't do that in production, but for a
demo/walkthrough the TLS keys are a little complicated.  The test suite and
production configurations do use TLS.

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
./etcd-manager --insecure --etcd-insecure --address 127.0.0.2 --etcd-address 127.0.0.2 --cluster-name=test --backup-store=file:///tmp/etcd-manager/backups/test --data-dir=/tmp/etcd-manager/data/test/2 --client-urls=http://127.0.0.2:4001 --quarantine-client-urls=http://127.0.0.2:8001 --peer-urls=http://127.0.0.2:2380
./etcd-manager --insecure --etcd-insecure --address 127.0.0.3 --etcd-address 127.0.0.3 --cluster-name=test --backup-store=file:///tmp/etcd-manager/backups/test --data-dir=/tmp/etcd-manager/data/test/3 --client-urls=http://127.0.0.3:4001 --quarantine-client-urls=http://127.0.0.3:8001 --peer-urls=http://127.0.0.3:2380
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
> ./etcd-manager-ctl -backup-store=file:///tmp/etcd-manager/backups/test get
etcd-manager-ctl
member_count:1 etcd_version:"2.2.1"
> ./etcd-manager-ctl -backup-store=file:///tmp/etcd-manager/backups/test -member-count=3 --etcd-version=2.2.1 configure-cluster
```

If you now bounce the first etcd-manager process (control-C and relaunch), the cluster will reconfigure itself.  This bouncing is typically done by a rolling-update, though etcd-manager can also be configured to automatically look for configuration changes:

```
> curl http://127.0.0.1:4001/v2/members/
{"members":[{"id":"c05691f8951bfaf5","name":"FD2NN12mS_K4ovji8dTL9g","peerURLs":["http://127.0.0.1:2380"],"clientURLs":["http://127.0.0.1:4001"]},{"id":"c3dd045ca41acb5f","name":"ykjVfcnf5DWUCwpKSBQFEg","peerURLs":["http://127.0.0.3:2380"],"clientURLs":["http://127.0.0.3:4001"]},{"id":"d0c3c4397fa244d1","name":"d7qDvoAwX5bhhUwQo0CHgg","peerURLs":["http://127.0.0.2:2380"],"clientURLs":["http://127.0.0.2:4001"]}]}
```

Play around with stopping & starting the nodes, or with removing the data for one at a time.  The cluster should be available whenever a majority of the processes are running,
and as long as you allow the cluster to recover before deleting a second data directory, no data should be lost.
(We'll cover "majority" disaster recovery later)

### Disaster recovery

The etcd-manager performs periodic backups.  In the event of a total failure, we
can restore from that backup.  Note that this involves data loss since the last
backup, so we require a manual trigger.

Verify backup/restore works correctly:

```
curl -XPUT -d "value=world"  http://127.0.0.1:4001/v2/keys/hello
```

* Wait for a "took backup" message (TODO: should we be able to force a backup?)
* Stop all 3 processes
* Remove the active data: `rm -rf /tmp/etcd-manager/data/test`
* Restart all 3 processes

A leader will be elected, and will start logging `etcd has 0 members registered; must issue restore-backup command to proceed`

List the available backups with:

```bash
> ./etcd-manager-ctl -backup-store=file:///tmp/etcd-manager/backups/test list-backups
2019-01-14T15:26:45Z-000001
2019-01-14T15:27:43Z-000001
2019-01-14T15:29:48Z-000001
2019-01-14T15:29:48Z-000002
2019-01-14T15:29:48Z-000003
```

Issue the restore-backup command:

```bash
> ./etcd-manager-ctl -backup-store=file:///tmp/etcd-manager/backups/test restore-backup 2019-01-14T15:29:48Z-000003
added restore-backup command: timestamp:1547480961703914946 restore_backup:<cluster_spec:<member_count:3 etcd_version:"2.2.1" > backup:"2019-01-14T15:29:48Z-000003" >
```

The controller will shortly restore the backup.  Confirm this with:

```bash
curl http://127.0.0.1:4001/v2/members/
curl http://127.0.0.1:4001/v2/keys/hello
```

### Upgrading

```
> ./etcd-manager-ctl -backup-store=file:///tmp/etcd-manager/backups/test get
member_count:3 etcd_version:"2.2.1"
> ./etcd-manager-ctl -backup-store=file:///tmp/etcd-manager/backups/test -member-count=3 --etcd-version=3.2.24 configure-cluster
```

Bounce the etcd-manager that has leadership so that it picks up the reconfiguration.

Dump keys to be sure that everything copied across:
```
> ETCDCTL_API=3 /opt/etcd-v3.2.24-linux-amd64/etcdctl --endpoints http://127.0.0.1:4001 get "" --prefix
/hello
world
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

```
./etcd-manager-ctl -backup-store=file:///tmp/etcd-manager/backups/test -member-count=3 --etcd-version=2.2.1 configure-cluster
```
