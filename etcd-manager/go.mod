module sigs.k8s.io/etcdadm/etcd-manager

go 1.16

replace k8s.io/kops => k8s.io/kops v1.21.0-alpha.3

// Version kubernetes-1.15.3
//replace k8s.io/kubernetes => k8s.io/kubernetes v1.19.0
//replace k8s.io/api => k8s.io/api kubernetes-1.19.0
//replace k8s.io/apimachinery => k8s.io/apimachinery kubernetes-1.19.0
//replace k8s.io/client-go => k8s.io/client-go kubernetes-1.19.0
//replace k8s.io/cloud-provider => k8s.io/cloud-provider kubernetes-1.19.0
//replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers kubernetes-1.19.0
//replace k8s.io/kubectl => k8s.io/kubectl kubernetes-1.19.0

replace k8s.io/kubernetes => k8s.io/kubernetes v1.21.0

replace k8s.io/api => k8s.io/api v0.21.0

replace k8s.io/apimachinery => k8s.io/apimachinery v0.21.0

replace k8s.io/client-go => k8s.io/client-go v0.21.0

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.21.0

replace k8s.io/kubectl => k8s.io/kubectl v0.21.0

replace k8s.io/mount-utils => k8s.io/mount-utils v0.21.0

// Dependencies we don't really need, except that kubernetes specifies them as v0.0.0 which confuses go.mod
//replace k8s.io/apiserver => k8s.io/apiserver kubernetes-1.19.0
//replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver kubernetes-1.19.0
//replace k8s.io/kube-scheduler => k8s.io/kube-scheduler kubernetes-1.19.0
//replace k8s.io/kube-proxy => k8s.io/kube-proxy kubernetes-1.19.0
//replace k8s.io/cri-api => k8s.io/cri-api kubernetes-1.19.0
//replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib kubernetes-1.19.0
//replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers kubernetes-1.19.0
//replace k8s.io/component-base => k8s.io/component-base kubernetes-1.19.0
//replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap kubernetes-1.19.0
//replace k8s.io/metrics => k8s.io/metrics kubernetes-1.19.0
//replace k8s.io/sample-apiserver => k8s.io/sample-apiserver kubernetes-1.19.0
//replace k8s.io/kube-aggregator => k8s.io/kube-aggregator kubernetes-1.19.0
//replace k8s.io/kubelet => k8s.io/kubelet kubernetes-1.19.0
//replace k8s.io/cli-runtime => k8s.io/cli-runtime kubernetes-1.19.0
//replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager kubernetes-1.19.0
//replace k8s.io/code-generator => k8s.io/code-generator kubernetes-1.19.0
//replace k8s.io/cli-runtime => k8s.io/cli-runtime kubernetes-1.19.0

replace k8s.io/apiserver => k8s.io/apiserver v0.21.0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.0

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.21.0

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.21.0

replace k8s.io/cri-api => k8s.io/cri-api v0.21.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.0

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.21.0

replace k8s.io/component-base => k8s.io/component-base v0.21.0

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.0

replace k8s.io/metrics => k8s.io/metrics v0.21.0

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.21.0

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.21.0

replace k8s.io/kubelet => k8s.io/kubelet v0.21.0

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.21.0

replace k8s.io/code-generator => k8s.io/code-generator v0.21.0

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.0

replace k8s.io/controller-manager => k8s.io/controller-manager v0.21.0

replace k8s.io/component-helpers => k8s.io/component-helpers v0.21.0

replace google.golang.org/grpc => google.golang.org/grpc v1.29.1

require (
	cloud.google.com/go v0.81.0
	github.com/Azure/azure-sdk-for-go v53.1.0+incompatible
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.7
	github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/aliyun/alibaba-cloud-sdk-go v1.61.1022
	github.com/aws/aws-sdk-go v1.38.17
	github.com/blang/semver v3.5.1+incompatible
	github.com/digitalocean/godo v1.59.0
	github.com/golang/protobuf v1.5.2
	github.com/gophercloud/gophercloud v0.17.0
	github.com/prometheus/client_golang v1.10.0
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200910180754-dd1b699fc489
	golang.org/x/net v0.0.0-20210410081132-afb366fc7cd1
	golang.org/x/oauth2 v0.0.0-20210402161424-2e8d93401602
	golang.org/x/tools v0.1.0 // indirect
	google.golang.org/api v0.44.0
	google.golang.org/grpc v1.36.1
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/yaml.v2 v2.4.0
	honnef.co/go/tools v0.0.1-2020.1.6
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.21.0
	k8s.io/klog/v2 v2.8.0
	k8s.io/kops v1.21.0-alpha.3
	k8s.io/mount-utils v0.21.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
)
