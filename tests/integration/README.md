# Sail Operator Integration Test

This integration test suite is similar to the upstream Istio integration tests. The scripts `lib.sh`, `kind_provisioner.sh` and `istio-integ-suite-kind.sh` are copied from `github.com/maistra/istio` repo.

## Pre-requisites

* To perform OCP integration testing, it is essential to have a functional OCP (OpenShift Container Platform) cluster already running. However, when testing against a KinD (Kubernetes in Docker) environment, the KinD cluster will be automatically configured using the provided script.

## How to Run it Manually

1. Create a builder container:

```
$ docker pull registry.ci.openshift.org/ci/maistra-builder:upstream-master
$ docker run -d -t --rm \
    --name test \
    --privileged \
    -v /var/lib/docker \
    registry.ci.openshift.org/ci/maistra-builder:upstream-master \
    entrypoint tail -f /dev/null
```

2. Copy this sail-operator integration tests source code into the container, copy kubeconfig file into container and copy oc binary into container (Only needed when you are running against OCP cluster):

```
$ cd $(git rev-parse --show-toplevel)
$ docker cp . test:/work
$ docker cp ~/.kube/ test:/
$ docker cp `which oc` test:/bin
```
*Note*: if you are running in arm64, you need to download the proper oc binary from the OCP cluster to be copied into the container. For example, if your architecture is arm64 and your os is macOs, you can download the oc binary from https://downloads-openshift-console.apps-crc.testing/arm64/linux/oc.tar and copy it into the container.

3. Run `operator-integ-suite.sh --flag` in the builder container using the proper flag using the make target. Valid flags are `--ocp` and `--kind`. 

* To run in OCP cluster:
```
$ docker exec -it test /bin/bash
git config --global --add safe.directory /work
oc login [OCP API server] --kubeconfig /work/ci-kubeconfig
cd work
(root@) make test.integration.ocp
```

* To run in kind cluster:
```
$ docker exec -it test /bin/bash
git config --global --add safe.directory /work
export KUBECONFIG=/work/ci-kubeconfig
cd work
(root@) make test.integration.kind
```