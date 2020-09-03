load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:proto disable_global
gazelle(
    name = "gazelle",
    external = "vendored",
    prefix = "sigs.k8s.io/etcdadm/etcd-manager",
)

[genrule(
    name = "etcd-v2.2.1-linux-amd64_%s" % c,
    srcs = ["@etcd_2_2_1_tar//file"],
    outs = ["etcd-v2.2.1-linux-amd64/%s" % c],
    cmd = "tar -x -z --no-same-owner -f ./$(location @etcd_2_2_1_tar//file) etcd-v2.2.1-linux-amd64/%s && mv etcd-v2.2.1-linux-amd64/%s \"$@\"" % (c, c),
    visibility = ["//visibility:public"],
) for c in [
    "etcd",
    "etcdctl",
]]

[genrule(
    name = "etcd-v3.1.12-linux-amd64_%s" % c,
    srcs = ["@etcd_3_1_12_tar//file"],
    outs = ["etcd-v3.1.12-linux-amd64/%s" % c],
    cmd = "tar -x -z --no-same-owner -f ./$(location @etcd_3_1_12_tar//file) etcd-v3.1.12-linux-amd64/%s && mv etcd-v3.1.12-linux-amd64/%s \"$@\"" % (c, c),
    visibility = ["//visibility:public"],
) for c in [
    "etcd",
    "etcdctl",
]]

[genrule(
    name = "etcd-v3.2.18-linux-amd64_%s" % c,
    srcs = ["@etcd_3_2_18_tar//file"],
    outs = ["etcd-v3.2.18-linux-amd64/%s" % c],
    cmd = "tar -x -z --no-same-owner -f ./$(location @etcd_3_2_18_tar//file) etcd-v3.2.18-linux-amd64/%s && mv etcd-v3.2.18-linux-amd64/%s \"$@\"" % (c, c),
    visibility = ["//visibility:public"],
) for c in [
    "etcd",
    "etcdctl",
]]

[genrule(
    name = "etcd-v3.2.24-linux-amd64_%s" % c,
    srcs = ["@etcd_3_2_24_tar//file"],
    outs = ["etcd-v3.2.24-linux-amd64/%s" % c],
    cmd = "tar -x -z --no-same-owner -f ./$(location @etcd_3_2_24_tar//file) etcd-v3.2.24-linux-amd64/%s && mv etcd-v3.2.24-linux-amd64/%s \"$@\"" % (c, c),
    visibility = ["//visibility:public"],
) for c in [
    "etcd",
    "etcdctl",
]]

[genrule(
    name = "etcd-v3.3.10-linux-amd64_%s" % c,
    srcs = ["@etcd_3_3_10_tar//file"],
    outs = ["etcd-v3.3.10-linux-amd64/%s" % c],
    cmd = "tar -x -z --no-same-owner -f ./$(location @etcd_3_3_10_tar//file) etcd-v3.3.10-linux-amd64/%s && mv etcd-v3.3.10-linux-amd64/%s \"$@\"" % (c, c),
    visibility = ["//visibility:public"],
) for c in [
    "etcd",
    "etcdctl",
]]

[genrule(
    name = "etcd-v3.3.13-linux-amd64_%s" % c,
    srcs = ["@etcd_3_3_13_tar//file"],
    outs = ["etcd-v3.3.13-linux-amd64/%s" % c],
    cmd = "tar -x -z --no-same-owner -f ./$(location @etcd_3_3_13_tar//file) etcd-v3.3.13-linux-amd64/%s && mv etcd-v3.3.13-linux-amd64/%s \"$@\"" % (c, c),
    visibility = ["//visibility:public"],
) for c in [
    "etcd",
    "etcdctl",
]]

[genrule(
    name = "etcd-v3.3.17-linux-amd64_%s" % c,
    srcs = ["@etcd_3_3_17_tar//file"],
    outs = ["etcd-v3.3.17-linux-amd64/%s" % c],
    cmd = "tar -x -z --no-same-owner -f ./$(location @etcd_3_3_17_tar//file) etcd-v3.3.17-linux-amd64/%s && mv etcd-v3.3.17-linux-amd64/%s \"$@\"" % (c, c),
    visibility = ["//visibility:public"],
) for c in [
    "etcd",
    "etcdctl",
]]

[genrule(
    name = "etcd-v3.4.3-linux-amd64_%s" % c,
    srcs = ["@etcd_3_4_3_tar//file"],
    outs = ["etcd-v3.4.3-linux-amd64/%s" % c],
    cmd = "tar -x -z --no-same-owner -f ./$(location @etcd_3_4_3_tar//file) etcd-v3.4.3-linux-amd64/%s && mv etcd-v3.4.3-linux-amd64/%s \"$@\"" % (c, c),
    visibility = ["//visibility:public"],
) for c in [
    "etcd",
    "etcdctl",
]]
