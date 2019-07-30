# How backup restore works

This document attempts to describe how the backup restore procedure works - under the hood - in `etcd-manager`.


## Quarantined state

In a `etcd` cluster managed by `etcd-manager`, a peer has two different client URLs in its configuration:
- `client_urls`: URL used by external clients to connect to the `etcd` cluster
- `quarantined_client_urls`: URL used to temporarily run the `etcd` node in a quarantined state, where external clients can't connect (because configured with the `client_urls`) but the other `etcd-manager` peers can eventually connect via the quarantined urls

On a normal condition, `etcd-manager` will start the `etcd` server listening on `client_urls`, where external clients will connect to. However, during special operations, `etcd-manager` can start the `etcd` server in a **quarantined** state and it will listen on `quarantined_client_urls` (different ports) so that external clients will not be able to connect. This technique is used to introduce a sort of fence at network level during special operations executed by `etcd-manager`, like version upgrade or backups restore.


## How the backup restore works

The `etcd-manager` continuously runs a reconcile loop. If the node has the leadership across peers and has read a `restore-backup` command from the storage, it tries to run it. However, until the required number of peers is reached, the command won't be executed and will log:

```
I0619 10:54:59.700623    9271 controller.go:380] got restore-backup command: timestamp:TIME restore_backup:DATA
I0619 10:54:59.700669    9271 controller.go:389] insufficient peers in our gossip group to build a cluster of size 3
```

Once the last `etcd-manager` starts and joins the cluster peers, it will get the leadership (the last node to join gets the leadership), will read the `restore-backup` command from storage and will run it. To restore a backup, `etcd-manager` does:

1. Stops `etcd` on other peers (but not self)
   - Sends a `StopEtcd` gPRC command
   - Waits the response from each peer
    ```
    I0619 10:56:02.044224   11192 controller.go:380] got restore-backup command: timestamp:TIME restore_backup:DATA
    I0619 10:56:02.146643   11192 newcluster.go:111] stopped etcd on peer "etcd-b":
    I0619 10:56:02.249354   11192 newcluster.go:111] stopped etcd on peer "etcd-c":
    ```

2. Creates a new `etcd` cluster
   - Generates a **new random cluster token**
   - Sends a `JoinCluster` gRPC command to all peers (including self) with `Phase` to set to `Phase_PHASE_PREPARE` and the new cluster token
     ```
     I0619 10:56:02.273360   11192 newcluster.go:120] starting new etcd cluster with DATA
     ```
   - Waits the response from each peer
    ```
    I0619 10:56:02.273850   11192 newcluster.go:137] JoinClusterResponse:
    I0619 10:56:02.274808   11192 newcluster.go:137] JoinClusterResponse:
    I0619 10:56:02.275990   11192 newcluster.go:137] JoinClusterResponse:
    ```
   - Sends a `JoinCluster` gRPC command to all peers (including self) with `Phase` to set to `Phase_PHASE_INITIAL_CLUSTER` and the new cluster token
   - Each peer will start `etcd` in quarantined state
   - Waits the response from each peer
    ```
    I0619 10:56:02.740260   11192 newcluster.go:155] JoinClusterResponse:
    I0619 10:56:03.308194   11192 newcluster.go:155] JoinClusterResponse:
    I0619 10:56:04.227142   11192 newcluster.go:155] JoinClusterResponse:
    ```

3. Sends a `DoRestore` command to the first healthy peer
   - Picks the first healthy node in list of peers
   - Sends a `DoRestore` gRPC command
   - Waits the response from the peer


The `etcd-manager` peer which has received the `DoRestore` command:

1. Loads the backup `.meta` file from the storage (ie. S3)
    ```
    I0619 10:56:04.284105   10845 vfs.go:118] Loading info for backup "TIMESTAMP"
    I0619 10:56:04.337348   10845 vfs.go:132] read backup info for "TIMESTAMP": etcd_version:"3.2.24" timestamp:1560935018 cluster_spec:<member_count:3 etcd_version:"3.2.24" >
    ```
2. Downloads the backup from the storage and extracts it
    ```
    I0619 10:56:04.337395   10845 restore.go:108] Downloading backup "TIMESTAMP" to /tmp/restore-etcd-TIMESTAMP/download/snapshot.db.gz
    I0619 10:56:04.337401   10845 vfs.go:142] Downloading backup "TIMESTAMP" -> /tmp/restore-etcd-TIMESTAMP/download/snapshot.db.gz
    I0619 10:56:04.337471   10845 s3fs.go:220] Reading file "s3://BUCKET/PATH/backups/etcd/main/TIMESTAMP/etcd.backup.gz"
    ```
3. Runs `etcdctl snapshot restore` command
    ```
    I0619 10:56:05.437759   10845 restore.go:195] restoring snapshot
    I0619 10:56:05.519188   10845 etcdprocess.go:392] snapshot restore complete
    ```
4. Runs a new `etcd` instance to read from the backup
    ```
    I0619 10:56:05.519239   10845 restore.go:208] starting etcd to read backup
    I0619 10:56:05.519256   10845 etcdprocess.go:172] executing command /opt/etcd-v3.2.24-linux-amd64/etcd [/opt/etcd-v3.2.24-linux-amd64/etcd --force-new-cluster]
    I0619 10:56:06.537803   10845 restore.go:233] copying etcd keys from backup-restore process to new cluster
    [...]
    I0619 10:56:08.240120   10845 restore.go:238] restored 413 keys
    ```
5. Once the restore has completed, the new instance - used to read from the backup - is stopped
    ```
    I0619 10:56:08.240145   10845 restore.go:65] stopping etcd that was reading backup
    ```

At this point, the `etcd-manager` leader which has issued the `DoRestore` command will receive the `DoRestoreResponse` and:
1. Deletes the `restore-backup` command from storage (ie. S3) to avoid the command will be executed again
2. Triggers a backup of current cluster (on the leader node the `etcd` server is still running on the pre-restore data because the cluster token has not been switched yet)
3. Sends a `Reconfigure` gRPC command to all peers to exit quarantine. This command will be also received by the leader node itself
    ```
    I0619 10:56:09.158178   11192 restore.go:82] Setting quarantined state to false
    ```
4. Each peer (including the leader) will restart `etcd`:
    - Stop `etcd` server
    - Update the `etcd` cluster state (will switch to a new cluster token)
    - Start `etcd` server
5. Waits the response from all peers
    ```
    I0619 10:56:09.362085   11192 restore.go:100] ReconfigureResponse:
    I0619 10:56:09.879961   11192 restore.go:100] ReconfigureResponse:
    I0619 10:56:10.193554   11192 restore.go:100] ReconfigureResponse:
    ```

Finally, the `etcd-manager` node which has executed the backup restore will have the new data, which will be synced to the other `etcd` nodes (having an empty data store because the procedure has created a new cluster).
