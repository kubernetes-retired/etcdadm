etcdadm
=======

etcdadm is a command-line tool for managing an etcd cluster. It makes it easy to create a new cluster, add a member to, or remove a member from an existing cluster. Its user experience is inspired by [kubeadm](https://kubernetes.io/docs/reference/setup-tools/kubeadm/).

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