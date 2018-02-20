# The backup structure

We hope that the backup structure used by etcd-manager and etcd-backup can be adopted by all tools,
so that there can be portability between etcd backups.

As such, the format is intended to be minimal and hopefully "unsuprising"

## Overview

Backups are stored in a filesystem like structure.  The etcd data itself is backed up using the appropriate etcd backup tool,
a snapshot for v3, and a backup directory tree for v2.  Then a single file `etcd.backup.tgz` is created for the backup data.
For V2 this is a tarfile of the backup directory, and for V3 this simply a gzip compressed version of the snapshot.

Alongside the data file, a backup program _should_ write a JSON file `_etcd_backup.meta`, containing
at least a root key `etcdVersion` with the etcd version that wrote the backup.
This lets the program restoring a backup know how to read the backup data.

etcd-manager writes additional keys into the `_etcd_backup.meta`: a `clusterSpec` which is still evolving, but
includes a specification of the number of cluster members, for example.  This allows self-organizing etcd clusters to
bootstrap.

(An open question is whether we should write the metadata into the tar file instead.  The problem with that is that the golang
tar writer doesn't make it easy to stream a tarfile when we don't know the length of the entries)

## Backup naming

Each backup is stored in a directory (however that is meaningful in the filesystem we are targeting) that is the child
of the base path.

Each directory should be named like `2018-01-30T01:02:03Z-000009`, which is a timestamp in what go calls `time.RFC3339` format,
followed by a dash, followed by a suffix string.  The suffix string can be used however is meaningful for the application.
A backup application _should_ try to avoid collisions by using a fixed first N characters of the suffix for something ordered,
for example the number of milliseconds or a sequence counter.

(`etcd-manager` currently keeps a per-process counter, so that even if we have multiple backups within the same second, they
should not collide.  In future we may use the etcd term & index position.)

(We're assuming roughly synchronized clocks here, which is why it might be better to make the term & index the primary sort key)