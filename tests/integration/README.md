# Maistra Istio Operator Integration Test

This integration test suite utilizes Ginkgo, a testing framework known for its expressive specs (reference: https://onsi.github.io/ginkgo/). The setup for the test run is similar to the upstream Istio integration tests:
* In the case of kind execution, it relies on the upstream script `kind_provisioner.sh` and `integ-suite-kind.sh`, which are copied from the `github.com/maistra/istio` repository to set up the kind cluster used for the test.
* In the case of OCP execution, it relies on the `inter-suite-ocp.sh` and `common-operator-integ-suite` scripts to setup the the OCP cluster to be ready for the test.

## Pre-requisites

* To perform OCP integration testing, it is essential to have a functional OCP (OpenShift Container Platform) cluster already running. However, when testing against a KinD (Kubernetes in Docker) environment, the KinD cluster will be automatically configured using the provided script.

## How to Run the test

* To run the integration tests in OCP cluster, use the following command:
```
$ make test.integration.ocp
```

* To run the integration tests in KinD cluster, use the following command:
```
$ make test.integration.kind
```

Both targets will run setup first by using `integ-suite-ocp.sh` and `integ-suite-kind.sh` scripts respectively, and then run the integration tests using the `common-operator-integ-suite` script setting different flags for OCP and KinD.

Note: By default, the test runs inside a container because the env var `BUILD_WITH_CONTAINER` default value is 1. Take into account that to be able to run the integration tests in a container, you need to have `docker` or `podman` installed in your machine. To select the container cli you will also need to set the `CONTAINER_CLI` env var to `docker` or `podman` in the `make` command, the default value is `docker`.

## Running the test locally

To run the integration tests without a container, use the following command:

```
$ make BUILD_WITH_CONTAINER=0 test.integration.kind
```
or
```
$ make BUILD_WITH_CONTAINER=0 test.integration.ocp
```

## Settings for integration test execution

The following environment variables define the behavior of the test run:

* SKIP_BUILD=false - If set to true, the test will skip the build process and an existing operator image will be used to deploy the operator and run the test. The operator image that is going to be used is defined by the `IMAGE` variable.
* IMAGE=quay.io/maistra-dev/istio-operator:latest - The operator image to be used to deploy the operator and run the test. This is useful when you want to test a specific operator image.
* SKIP_DEPLOY=false - If set to true, the test will skip the deployment of the operator. This is useful when the operator is already deployed in the cluster and you want to run the test only.
* OCP=false - If set to true, the test will set specific setup for OCP clusters and use `oc` command as default command. If set to false, the test will run in KinD cluster and use `kubectl` as command to interact with the cluster.
* NAMESPACE=istio-operator - The namespace where the operator will be deployed and the test will run.
* CONTROL_PLANE_NS=istio-system - The namespace where the control plane will be deployed.
* DEPLOYMENT_NAME=istio-operator - The name of the operator deployment.