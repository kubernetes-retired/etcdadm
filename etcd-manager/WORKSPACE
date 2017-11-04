http_archive(
    name = "io_bazel_rules_go",
    url = "https://github.com/bazelbuild/rules_go/releases/download/0.7.0/rules_go-0.7.0.tar.gz",
    sha256 = "91fca9cf860a1476abdc185a5f675b641b60d3acf0596679a27b580af60bf19c",
)

load("@io_bazel_rules_go//go:def.bzl", "go_rules_dependencies", "go_register_toolchains")
go_rules_dependencies()
go_register_toolchains()

load("@io_bazel_rules_go//proto:def.bzl", "proto_register_toolchains")
proto_register_toolchains()


#=============================================================================

# for building docker base images
debs = (
    (
        "deb_busybox",
        "83d809a22d765e52390c0bc352fe30e9d1ac7c82fd509e0d779d8289bfc8a53d",
        "http://ftp.us.debian.org/debian/pool/main/b/busybox/busybox-static_1.22.0-9+deb8u1_amd64.deb",
    ),
    (
        "deb_libc",
        "2d8de90c084a26c266fa8efa91564f99b2373a7949caa9a1db83460918e6e832",
        "http://ftp.us.debian.org/debian/pool/main/g/glibc/libc6_2.19-18+deb8u7_amd64.deb",
    ),
    ("deb_iptables", "7747388a97ba71fede302d70361c81d486770a2024185514c18b5d8eab6aaf4e", "http://ftp.us.debian.org/debian/pool/main/i/iptables/iptables_1.4.21-2+b1_amd64.deb"),
    ("deb_libnfnetlink0", "5d486022cd9e047e9afbb1617cf4519c0decfc3d2c1fad7e7fe5604943dbbf37", "http://ftp.us.debian.org/debian/pool/main/libn/libnfnetlink/libnfnetlink0_1.0.1-3_amd64.deb"),
    ("deb_libxtables10", "6783f316af4cbf3ada8b9a2b7bb5f53a87c0c2575c1903ce371fdbd45d3626c6", "http://ftp.us.debian.org/debian/pool/main/i/iptables/libxtables10_1.4.21-2+b1_amd64.deb"),
)

[http_file(
    name = name,
    sha256 = sha256,
    url = url,
) for name, sha256, url in debs]
