etcdadm
=======

etcdadm is a command-line tool for managing an etcd cluster. It makes it easy to create a new cluster, add a member to, or remove a member from an existing cluster. Its user experience is inspired by [kubeadm](https://kubernetes.io/docs/reference/setup-tools/kubeadm/).

## Table of Contents

  - [Getting Started](#getting-started)
    - [Installing](#installing)
    - [Creating a new cluster](#creating-a-new-cluster)
    - [Adding a member](#adding-a-member)
    - [Removing a member](#removing-a-member)
  - [Advanced Usage](#advanced-usage)
    - [Creating a new cluster from a snapshot](#creating-a-new-cluster-from-a-snapshot)
  - [Caveats & Limitations](#caveats--limitations)

## Getting Started

### Building

```
go get -u github.com/platform9/etcdadm
```

### Creating a new cluster

Copy `etcdadm` to each machine that will become a member. On the first machine, run:

```
etcdadm init
```

### Adding a member

1. Copy CA cert/key from a machine in the cluster
2. Choose a cluster endpoint (i.e. client URL of some member) and run `etcdadm join <endpoint>`

### Removing a member

1. Run `etcdadm reset`

## Advanced Usage

### Creating a new cluster from a snapshot

If you have an existing etcd snapshot, you can use it to create a new cluster:

```
etcdadm init --snapshot /path/to/etcd.snapshot
```

## Caveats and Limitations

1. Must run as root. (This is because etcdadm creates a systemd service)
2. Does not support etcd v2.