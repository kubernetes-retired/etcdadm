FROM golang:1.19-buster

RUN apt-get update
RUN apt-get upgrade -y

WORKDIR /etcd-manager-repo/

COPY go.* ./

RUN go mod download

COPY . ./

RUN go version

RUN go build -o /etcd-manager ./cmd/etcd-manager

#CMD /bin/sh
CMD tail -F /dev/null