load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:proto disable_global
# gazelle:exclude tools/deb-tools/
gazelle(
    name = "gazelle",
    external = "vendored",
    prefix = "sigs.k8s.io/etcdadm/etcd-manager",
)

load("//images:etcd.bzl", "supported_etcd_arch_and_version")

[
    genrule(
        name = "etcd-v{version}-linux-{arch}_{bin}".format(
            version = version,
            arch = arch,
            bin = bin,
        ),
        srcs = ["@etcd_{version}_{arch}_tar//file".format(
            version = version,
            arch = arch,
            bin = bin,
        )],
        outs = ["etcd-v{version}-linux-{arch}/{bin}".format(
            version = version,
            arch = arch,
            bin = bin,
        )],
        cmd = "tar -x -z --no-same-owner -f ./$(location @etcd_{version}_{arch}_tar//file) etcd-v{version}-linux-{arch}/{bin} && mv etcd-v{version}-linux-{arch}/{bin} \"$@\"".format(
            version = version,
            arch = arch,
            bin = bin,
        ),
        visibility = ["//visibility:public"],
    )
    for (arch, version) in supported_etcd_arch_and_version()
    for (bin) in [
        "etcd",
        "etcdctl",
    ]
]
