| :exclamation:  Issues for this repository are disabled |
|--------------------------------------------------------|
| Issues for OpenShift Service Mesh are tracked in Red Hat Jira. Please head to the [OSSM Jira project](https://issues.redhat.com/browse/OSSM) in order to browse or open an issue |

# Maistra Istio Operator

This project is an operator that can be used to manage the installation of an [Istio](https://istio.io) control plane.

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Deploy the operator to the cluster:

```sh
make deploy
```

2. Create an IstioControlPlane instance to install istiod:

```sh
kubectl apply -f config/samples/operator.istio.io_v1alpha1_istiocontrolplane.yaml
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
We welcome community contributions! For features or bugfixes, please first create an issue in our [OSSM Jira project](https://issues.redhat.com/browse/OSSM) and make sure to prefix your commit message with the issue ID.

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

### Writing Tests
Please try to keep business logic in separate packages that can be independently tested wherever possible, especially if you can avoid the usage of Kubernetes clients. It greatly simplifies testing if we don't need to use envtest everywhere.

E2E tests should use the ginkgo-style BDD testing method, an example can be found in [`controllers/istiocontrolplane_controller_test.go`](https://github.com/maistra/istio-operator/blob/maistra-3.0/controllers/istiocontrolplane_controller_test.go) for the test code and suite setup in [`controllers/suite_test.go`](https://github.com/maistra/istio-operator/blob/maistra-3.0/controllers/suite_test.go). All other tests should use standard golang xUnit-style tests (see [`pkg/kube/finalizers_test.go`](https://github.com/maistra/istio-operator/blob/maistra-3.0/pkg/kube/finalizers_test.go) for an example).

### OCP Integration Tests
Must be logged into OCP using 'oc' client
```sh
make test.integration.ocp
```