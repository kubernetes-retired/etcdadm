module kope.io/etcd-manager

go 1.12

replace k8s.io/kops => k8s.io/kops v1.16.3-0.20200529024915-348cc87883a5

// Version kubernetes-1.15.3
//replace k8s.io/kubernetes => k8s.io/kubernetes v1.15.3
//replace k8s.io/api => k8s.io/api kubernetes-1.15.3
//replace k8s.io/apimachinery => k8s.io/apimachinery kubernetes-1.15.3
//replace k8s.io/client-go => k8s.io/client-go kubernetes-1.15.3
//replace k8s.io/cloud-provider => k8s.io/cloud-provider kubernetes-1.15.3
//replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers kubernetes-1.15.3

replace k8s.io/kubernetes => k8s.io/kubernetes v1.15.3

replace k8s.io/api => k8s.io/api v0.0.0-20190819141258-3544db3b9e44

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190819141724-e14f31a72a77

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190819145148-d91c85d212d5

// Dependencies we don't really need, except that kubernetes specifies them as v0.0.0 which confuses go.mod
//replace k8s.io/apiserver => k8s.io/apiserver kubernetes-1.15.3
//replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver kubernetes-1.15.3
//replace k8s.io/kube-scheduler => k8s.io/kube-scheduler kubernetes-1.15.3
//replace k8s.io/kube-proxy => k8s.io/kube-proxy kubernetes-1.15.3
//replace k8s.io/cri-api => k8s.io/cri-api kubernetes-1.15.3
//replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib kubernetes-1.15.3
//replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers kubernetes-1.15.3
//replace k8s.io/component-base => k8s.io/component-base kubernetes-1.15.3
//replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap kubernetes-1.15.3
//replace k8s.io/metrics => k8s.io/metrics kubernetes-1.15.3
//replace k8s.io/sample-apiserver => k8s.io/sample-apiserver kubernetes-1.15.3
//replace k8s.io/kube-aggregator => k8s.io/kube-aggregator kubernetes-1.15.3
//replace k8s.io/kubelet => k8s.io/kubelet kubernetes-1.15.3
//replace k8s.io/cli-runtime => k8s.io/cli-runtime kubernetes-1.15.3
//replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager kubernetes-1.15.3
//replace k8s.io/code-generator => k8s.io/code-generator kubernetes-1.15.3
//replace k8s.io/cli-runtime => k8s.io/cli-runtime kubernetes-1.15.3

replace k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190819142446-92cc630367d0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190819143637-0dbe462fe92d

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20190819144657-d1a724e0828e

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20190819144346-2e47de1df0f0

replace k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190817025403-3ae76f584e79

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20190819145328-4831a4ced492

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20190819145509-592c9a46fd00

replace k8s.io/component-base => k8s.io/component-base v0.0.0-20190819141909-f0f7c184477d

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190819145008-029dd04813af

replace k8s.io/metrics => k8s.io/metrics v0.0.0-20190819143841-305e1cef1ab1

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20190819143045-c84c31c165c4

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190819142756-13daafd3604f

replace k8s.io/kubelet => k8s.io/kubelet v0.0.0-20190819144524-827174bad5e8

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20190819144832-f53437941eef

replace k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190612205613-18da4a14b22b

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190819144027-541433d7ce35

require (
	cloud.google.com/go v0.34.0
	github.com/aliyun/alibaba-cloud-sdk-go v1.60.290
	github.com/aws/aws-sdk-go v1.30.16
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/etcd v3.3.13+incompatible
	github.com/denverdino/aliyungo v0.0.0-20180409134525-ed76a3738861 // indirect
	github.com/digitalocean/godo v1.19.0
	github.com/go-ini/ini v1.32.0 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/gophercloud/gophercloud v0.0.0-20190216224116-dcc6e84aef1b
	github.com/pkg/sftp v0.0.0-20180127012644-738e088bbd93 // indirect
	golang.org/x/net v0.0.0-20200202094626-16171245cfb2
	golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a
	google.golang.org/api v0.0.0-20181220000619-583d854617af
	google.golang.org/grpc v1.17.0
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/ini.v1 v1.51.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.2.4
	honnef.co/go/tools v0.0.1-2019.2.3
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog v1.0.0
	k8s.io/kops v0.0.0-00010101000000-000000000000
	k8s.io/utils v0.0.0-20191114200735-6ca3b61696b6
)
