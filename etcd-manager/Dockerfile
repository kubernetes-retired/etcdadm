FROM golang:1.19-buster

RUN apt-get update
RUN apt-get upgrade -y

#ENV SCW_ACCESS_KEY=SCW0CTBHGDHJF9WSM9BD
#ENV SCW_SECRET_KEY=be593a72-d86d-48bf-9efb-8e7492d09811

WORKDIR /etcd-manager-repo/

COPY go.* ./

RUN go mod download

COPY . ./

RUN go version

RUN go build -o /etcd-manager ./cmd/etcd-manager

#CMD /bin/sh
#CMD tail -F /dev/null
#CMD /bin/sh -c "mkfifo /tmp/pipe; (tee -a /var/log/etcd.log < /tmp/pipe & ) ; ls / ; exec /etcd-manager --backup-store=scw://fake-state-store/plouf.fr/backups/etcd/events --client-urls=https://__name__:4002 --cluster-name=etcd-events --containerized=true --dns-suffix=.internal.plouf.fr --grpc-port=3997 --peer-urls=https://__name__:2381 --quarantine-client-urls=https://__name__:3995 --v=6 --volume-name-tag=k8s.io/etcd/events --volume-provider=scaleway --volume-tag=KubernetesCluster=plouf.fr --volume-tag=k8s.io/etcd/events --volume-tag=k8s.io/role/master=1 --volume-tag=volume=events > /tmp/pipe 2>&1"
CMD /bin/sh -c "mkfifo /tmp/pipe; (tee -a /var/log/etcd.log < /tmp/pipe & ) ; ls / ; exec /etcd-manager --backup-store=scw://kops-state-store-test/cluster.leila.sieben.fr/backups/etcd/events --client-urls=https://__name__:4002 --cluster-name=etcd-events --containerized=true --dns-suffix=.internal.cluster.leila.sieben.fr --grpc-port=3997 --peer-urls=https://__name__:2381 --quarantine-client-urls=https://__name__:3995 --v=6 --volume-name-tag=k8s.io/etcd/events --volume-provider=scaleway --volume-tag=KubernetesCluster=cluster.leila.sieben.fr --volume-tag=KubernetesCluster=cluster.leila.sieben.fr --volume-tag=k8s.io/etcd/events --volume-tag=k8s.io/etcd/events --volume-tag=k8s.io/role/master=1 --volume-tag=volume=events > /tmp/pipe 2>&1"
