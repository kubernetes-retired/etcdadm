@startuml
title etcdadm init

skinparam BoxPadding 20

participant user

box host1
participant etcdadm
participant filesystem
participant systemd
database member1
end box

participant "remote repo"

user->etcdadm: init

group download (if not in cache)
etcdadm->"remote repo": download etcd, etcdctl
etcdadm->filesystem: write etcd, etcdctl to local cache
end

group install artifacts
etcdadm->filesystem: copy etcd, etcdctl from local cache to install directory
end

group create certificates
etcdadm->filesystem: create CA certificate
etcdadm->filesystem: create client server/client certificates
etcdadm->filesystem: create peer server/client certificates
end

group create etcd systemd service
etcdadm->filesystem: create etcd systemd service
etcdadm->filesystem: create etcd environment
end

etcdadm->systemd: enable etcd service
etcdadm->systemd: start etcd service

create member1
systemd->member1: start

hide footbox
@enduml
