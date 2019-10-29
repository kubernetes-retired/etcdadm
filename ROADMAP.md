# Roadmap

The project's major goals, ranked by importance:

1. Define API types.
2. Read configuration from a file.
3. Integrate with the [Cluster API project](https://cluster-api.sigs.k8s.io/).
	- Support the "external etcd" use case.
	- Work together with [kubeadm](https://kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm/), which the Cluster API project uses in its default implementation.
4. Implement backup/restore functionality.
	- Give an example of how to take periodic backups automatically using etcdadm.
5. Implement "self-driving" functionality.
6. Support etcd [non-voting members](https://etcd.io/docs/v3.4.0/learning/), introduced in etcd v3.4.
