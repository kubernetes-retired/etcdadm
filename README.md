etcdadm
=======

etcdadm is a command-line tool for operating an etcd cluster. It makes it easy to create a new cluster, add a member to, or remove a member from an existing cluster. Its user experience is inspired by [kubeadm](https://kubernetes.io/docs/reference/setup-tools/kubeadm/).

[![asciicast](https://asciinema.org/a/Ham0ZE4YobtgfJTvcfn1SKfZN.png)](https://asciinema.org/a/Ham0ZE4YobtgfJTvcfn1SKfZN)

## Table of Contents

  - [Getting Started](#getting-started)
    - [Installing](#installing)
    - [Creating a new cluster](#creating-a-new-cluster)
    - [Adding a member](#adding-a-member)
    - [Removing a member](#removing-a-member)
  - [Advanced Usage](#advanced-usage)
    - [Creating a new cluster from a snapshot](#creating-a-new-cluster-from-a-snapshot)
  - [Caveats & Limitations](#caveats--limitations)
  - [Design]

## Getting Started

### Building

```
go get -u github.com/platform9/etcdadm
```

### Creating a new cluster

1. Copy `etcdadm` to each machine that will become a member.
2. Choose one machine and run

```
etcdadm init
```

### Adding a member

1. Copy the CA certificate and key from any machine in the cluster to the machine being added.
2. Choose a cluster endpoint (i.e. client URL of some member) and run

```
etcdadm join <endpoint>
```

### Removing a member

On the machine being removed, run

```
etcdadm reset
```

## Advanced Usage

### Creating a new cluster from a snapshot

If you have an existing etcd snapshot, you can use it to create a new cluster:

```
etcdadm init --snapshot /path/to/etcd.snapshot
```

## Caveats and Limitations

1. Must run as root. (This is because etcdadm creates a systemd service)
2. Does not support etcd v2.
3. Currently tested on Container Linux, with plans for other platforms.

## Design

Etcdadm is meant to simplify some of the more mundane steps of operating an etcd cluster. It will download a specific release, install the binary, configure a systemd service, and add/remove a cluster member. It is used today to operate etcd clusters in an environment with no IaaS APIs and no egress to the internet.

It does not automate cluster operation. For example, if a member permanently fails, and the operator cannot invoke `etcdadm reset` on that machine, the operator must use the etcd API to delete the failed member from the list of members.

On the other hand, etcdadm could be used with a higher-level controller that reconciles cluster membership against some external source of truth (e.g. an IaaS API like AWS EC2). The controller would delegate all of the tasks local to the host (installation, configuration) to etcdadm.