module github.com/maistra/istio-operator

go 1.15

require (
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/Masterminds/semver v1.5.0
	github.com/Masterminds/sprig v2.22.0+incompatible // indirect
	github.com/containerd/containerd v1.4.8 // indirect
	github.com/containerd/typeurl v0.0.0-20190228175220-2a93cfde8c20 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/emicklei/go-restful v2.11.1+incompatible // indirect
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.2.1
	github.com/goccy/go-yaml v1.8.8
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.5.2
	github.com/gregjones/httpcache v0.0.0-20181110185634-c63ab54fda8f // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.1.0
	github.com/magiconair/properties v1.8.1
	github.com/mikefarah/yq/v4 v4.6.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/opencontainers/runtime-spec v1.0.0 // indirect
	github.com/openshift/api v3.9.1-0.20190924102528-32369d4db2ad+incompatible
	github.com/openshift/library-go v0.0.0-20200214084717-e77ad9dd8ebd
	github.com/operator-framework/operator-sdk v0.18.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	go.uber.org/zap v1.14.1
	golang.org/x/crypto v0.0.0-20200709230013-948cd5f35899
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect
	gomodules.xyz/jsonpatch/v2 v2.0.1
	k8s.io/api v0.19.3
	k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.19.3
	k8s.io/helm v2.16.7+incompatible
	k8s.io/kubectl v0.18.3
	k8s.io/utils v0.0.0-20200603063816-c1c6865ac451
	maistra.io/api v0.0.0-20210601141927-1cbee4cb8ce8
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/yaml v1.2.0
)

replace vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm

replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190924102528-32369d4db2ad // Required until https://github.com/operator-framework/operator-lifecycle-manager/pull/1241 is resolved

// not sure why this is required, maybe scanning of yaml files?
replace istio.io/api => istio.io/api v0.0.0-20191111210003-35e06ef8d838

// fix autorest ambiguous import error
replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.3

replace k8s.io/client-go => k8s.io/client-go v0.18.3

replace k8s.io/api => k8s.io/api v0.18.3

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.3

replace k8s.io/apimachinery => k8s.io/apimachinery v0.18.3

// FIXME: https://issues.redhat.com/browse/OSSM-2367
replace k8s.io/code-generator => k8s.io/code-generator v0.18.3

replace github.com/go-logr/zapr => github.com/go-logr/zapr v0.2.0

// use json-patch 4.6.0+, since earlier versions cause json patch generation to be very slow (MAISTRA-1780)
// can't use 4.6.0-4.8.0, because it contains a go.mod file and thus can't be referenced by tag, so we use 4.9.0 (see https://github.com/evanphx/json-patch/pull/113)
replace github.com/evanphx/json-patch => github.com/evanphx/json-patch v4.9.0+incompatible

// pkg disappeard from bitbucket
replace bitbucket.org/ww/goautoneg => github.com/munnerz/goautoneg v0.0.0-20120707110453-a547fc61f48d

replace github.com/operator-framework/operator-sdk => github.com/maistra/operator-sdk v0.0.0-20210824135520-3b25565223d4

replace github.com/mikefarah/yaml/v2 => gopkg.in/yaml.v2 v2.4.0
