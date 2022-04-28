# Returns the supported arch & versions of etcd
# This avoids repeating the etcd versions in multiple places,
# and also avoids problems such as 3.1.12 not being available on arm
def supported_etcd_arch_and_version():
  return [
    [ "amd64", "3.1.12" ],
  ] + [
    (arch, version)
      for arch in ["amd64", "arm64"]
      for version in ["3.2.18", "3.2.24", "3.3.10", "3.3.13", "3.3.17", "3.4.3","3.4.13","3.5.0","3.5.1","3.5.3","3.5.4"]
  ]
