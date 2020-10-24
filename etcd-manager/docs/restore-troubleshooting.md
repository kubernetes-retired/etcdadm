# etcd restore troubleshooting

An etcd restore can sometimes leave your cluster in an incorrect state.
Below troubleshooting steps can help you resolve some of the issues that can occur.

## Api server slow

Shortly after an etcd restore, the api server may respond slowly, and you can see high cpu usage on your masters.
In this case, the api server is trying to restore the cluster to the correct state, which may require a lot of changes.

You can help this process by increasing the master's instance size and stopping unnecessary pod creation / API calls (CronJobs, automation scripts etc).
Rolling nodes may also help in some scenarios. Make sure you check for unhealthy pods and fix any potential issues,
as otherwise the api server may remain busy with these pods.

## Kubernetes service unavailable / flannel issues

In some scenarios, the in-cluster kubernetes service can become unavailable, or have spotty connectivity issues.
If you're using flannel for networking, this can mean that flannel won't start up on (some) nodes.

This issue is due to old master IPs being set as endpoints in the kubernetes service.
Removing the endpoints manually will not resolve the issue, as they will be added automatically again.

You can fix this by manually removing the old master IPs from etcd:

#### Get into a master instance

SSH into one of your master instances.

#### Download etcdctl
Download `etcdctl` (not `etcd-manager-ctl`) onto the master from the correct [etcd release](https://github.com/etcd-io/etcd/releases).
The easiest way to figure out the etcd version you're running, is by checking the etcd log (`/var/log/etcd.log`).

#### Retrieve the old master IPs

You can get the old master IPs by describing the kubernetes service (under Endpoints):

`kubectl describe svc kubernetes`

You will need to filter out the old IPs yourself by checking what IPs correspond to running instances, and which ones don't.

#### Delete the old master IPs from etcd

Run the following command for all old master IPs:

`sudo ETCDCTL_API=3 ./etcdctl --endpoints=https://127.0.0.1:4001 --cacert /etc/kubernetes/pki/kube-apiserver/etcd-ca.crt --cert /etc/kubernetes/pki/kube-apiserver/etcd-client.crt --key /etc/kubernetes/pki/kube-apiserver/etcd-client.key del "/registry/masterleases/[MASTER IP]"`

Make sure to replace `[MASTER IP]` with the IP of the old master.

#### Kubernetes service connectivity back

Once you've removed the old master IPs from etcd, the kubernetes service endpoints should be automatically updated.
The kubernetes service should be functional again now, and flannel should come up on new nodes.

## Corrupted restore-backup command in storage

Once a cluster is established and a leader is selected, the leader will read the
`restore-backup` command from storage. If this is an invalid command -- for
example, one that specifies a nonexistent backup -- it is possible to enter a
state where `etcd-manager` will fail to start up, and eventually enter a
`CrashLoopBackOff` state.

You can fix this by manually deleting the command from storage. You can do this
by removing the backup via `etcd-manager-ctl delete-backup <backupname>`.
