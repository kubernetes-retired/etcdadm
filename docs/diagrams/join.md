@startuml
title etcdadm join https://member1:2379

skinparam BoxPadding 20

participant user

box host2
participant etcdadm
participant filesystem
participant systemd
database member2
end box

box host1
database member1
end box

participant "remote repo"

user->user: copy CA cert/key to host
note over user
  all client and peer certificates
  must be signed by the same CA
end note
user->etcdadm: join https://member1:2379

group add new member
etcdadm->member1: add member
end

group download (if not in cache)
etcdadm->"remote repo": download etcd, etcdctl
etcdadm->filesystem: write etcd, etcdctl to local cache
end

group install artifacts
etcdadm->filesystem: copy etcd, etcdctl from local cache to install directory
end

group create certificates
etcdadm->filesystem: create client server/client certificates
etcdadm->filesystem: create peer server/client certificates
end

group create etcd systemd service
etcdadm->filesystem: create etcd systemd service
etcdadm->filesystem: create etcd environment
end

etcdadm->systemd: enable etcd service
etcdadm->systemd: start etcd service

create member2
systemd->member2: start

member2->member1: join

hide footbox
@enduml