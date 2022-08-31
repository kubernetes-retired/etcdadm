~/scaleway-cli/scw instance server create image=debian_buster type=DEV1-M zone=fr-par-1 \
    name="master-test" \
    tags.0="instance-group=master-test" \
    tags.1="KubernetesCluster=plouf.fr" ;
~/scaleway-cli/scw instance volume create size=40G volume-type=b_ssd zone=fr-par-1 \
    name="etcd-main-test" \
    tags.0="KubernetesCluster=plouf.fr" \
    tags.1="k8s.io/etcd/main" \
    tags.2="k8s.io/role/master=1" \
    tags.3="volume=main" ;
~/scaleway-cli/scw instance volume create size=40G volume-type=b_ssd zone=fr-par-1 \
    name="etcd-events-test" \
    tags.0="KubernetesCluster=plouf.fr" \
    tags.1="k8s.io/etcd/events" \
    tags.2="k8s.io/role/master=1" \
    tags.3="volume=events" ;

    /usr/bin/python3