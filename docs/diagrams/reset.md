@startuml
title etcdadm reset https://member1:2379

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

user->etcdadm: reset

group reconfigure etcd cluster
etcdadm->member1: remove member2
end

destroy member2
etcdadm->systemd: stop etcd service
etcdadm->systemd: disable etcd service

group uninstall artifacts
etcdadm->filesystem: delete etcd, etcdctl from install directory
end

group destroy certificates
etcdadm->filesystem: destroy CA certificate
etcdadm->filesystem: destroy client server/client certificates
etcdadm->filesystem: destroy peer server/client certificates
end

group destroy systemd service
etcdadm->filesystem: destroy etcd service
etcdadm->filesystem: destroy etcd environment
end

hide footbox
@enduml
