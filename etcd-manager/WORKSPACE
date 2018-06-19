http_archive(
    name = "io_bazel_rules_go",
    url = "https://github.com/bazelbuild/rules_go/releases/download/0.13.0/rules_go-0.13.0.tar.gz",
    sha256 = "ba79c532ac400cefd1859cbc8a9829346aa69e3b99482cd5a54432092cbc3933",
)

http_archive(
    name = "bazel_gazelle",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/0.13.0/bazel-gazelle-0.13.0.tar.gz"],
    sha256 = "bc653d3e058964a5a26dcad02b6c72d7d63e6bb88d94704990b908a1445b8758",
)

load("@io_bazel_rules_go//go:def.bzl", "go_rules_dependencies", "go_register_toolchains")
go_rules_dependencies()
go_register_toolchains()
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
gazelle_dependencies()

#=============================================================================

git_repository(
    name = "io_bazel_rules_docker",
    remote = "https://github.com/bazelbuild/rules_docker.git",
    tag = "v0.4.0",
)

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
    container_repositories = "repositories",
)

container_repositories()

# Use a kubernetes base image which includes ca-certificates (so we can talk to google for OAuth)
# Some potential here: https://github.com/GoogleCloudPlatform/base-images-docker
container_pull(
    name = "debian_base_amd64",
    digest = "sha256:cc782ed16599000ca4c85d47ec6264753747ae1e77520894dca84b104a7621e2",
    registry = "gcr.io",
    repository = "google-containers/debian-hyperkube-base-amd64",
    tag = "0.10",
)


#=============================================================================
# etcd rules
http_file(
    name = "etcd_2_2_1_tar",
    sha256 = "59f7985c81b6bc551246c165c2fd83e33a063875e4e0c61920b1d90a4910f462",
    url = "https://github.com/coreos/etcd/releases/download/v2.2.1/etcd-v2.2.1-linux-amd64.tar.gz",
)

http_file(
    name = "etcd_3_1_12_tar",
    sha256 = "4b22184bef1bba8b4908b14bae6af4a6d33ec2b91e4f7a240780e07fa43f2111",
    url = "https://github.com/coreos/etcd/releases/download/v3.1.12/etcd-v3.1.12-linux-amd64.tar.gz",
)

http_file(
    name = "etcd_3_2_18_tar",
    sha256 = "b729db0732448064271ea6fdcb901773c4fe917763ca07776f22d0e5e0bd4097",
    url = "https://github.com/coreos/etcd/releases/download/v3.2.18/etcd-v3.2.18-linux-amd64.tar.gz",
)
