# Maistra Istio Operator

This project is an operator that can be used to manage the installation of an [Istio](https://istio.io) control plane.

## Installation

The **Istio Operator** has a dependency on the **Jaeger Operator**, **Elasticsearch Operator**  and **Kiali Operator**.  Before installing the Istio Operator, please make sure the
Jaeger Operator, Elasticsearch Operator and Kiali Operator have been installed.

### Installing the Elasticsearch Operator

If available, the Elasticsearch operator (version 4.1) should be installed from the OperatorHub.

Alternatively, to install the Elasticsearch operator manually, execute the following commands:

```
# See note below for explanation of why kubectl is used instead of oc
kubectl create ns openshift-logging # create the project for the elasticsearch operator
oc create -f https://raw.githubusercontent.com/openshift/elasticsearch-operator/release-4.1/manifests/01-service-account.yaml -n openshift-logging
oc create -f https://raw.githubusercontent.com/openshift/elasticsearch-operator/release-4.1/manifests/02-role.yaml
oc create -f https://raw.githubusercontent.com/openshift/elasticsearch-operator/release-4.1/manifests/03-role-bindings.yaml
oc create -f https://raw.githubusercontent.com/openshift/elasticsearch-operator/release-4.1/manifests/04-crd.yaml -n openshift-logging
curl https://raw.githubusercontent.com/openshift/elasticsearch-operator/release-4.1/manifests/05-deployment.yaml | sed 's/latest/4.1/g' | oc create -n openshift-logging -f -
```

NOTE: It is necessary to use `kubectl` for the creation of the openshift-logging namespace, as the `oc` command will return the following error:
```
Error from server (Forbidden): project.project.openshift.io "openshift-logging" is forbidden: cannot request a project starting with "openshift-"
```


### Installing the Jaeger Operator

To install the Jaeger operator, execute the following commands:

```
oc new-project observability # create the project for the jaeger operator
oc create -n observability -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/v1.13.1/deploy/crds/jaegertracing_v1_jaeger_crd.yaml
oc create -n observability -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/v1.13.1/deploy/service_account.yaml
oc create -n observability -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/v1.13.1/deploy/role.yaml
oc create -n observability -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/v1.13.1/deploy/role_binding.yaml
oc create -n observability -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/v1.13.1/deploy/operator.yaml
```

### Installing the Kiali Operator

To install the Kiali operator, execute the following command:

```
bash <(curl -L https://git.io/getLatestKialiOperator) --operator-image-version v1.0.0 --operator-watch-namespace '**' --accessible-namespaces '**' --operator-install-kiali false
```

For more details on installing the Kiali operator, see the [Kiali documentaton](https://www.kiali.io/documentation/getting-started).

### Installing the Istio Operator

If `istio-operator` and `istio-system` projects/namespaces have not been created, create these projects first. For example:

```
$ oc new-project istio-operator
$ oc new-project istio-system
```

All resource definitions required to install the operator can be found in the [deploy](./deploy) directory, and can be
installed easily using your favorite Kubernetes command-line client.  You must have `cluster-admin` privileges to install
the operator.

```
$ oc apply -n istio-operator -f ./deploy/maistra-operator.yaml
```

or

```
$ kubectl apply -n istio-operator -f ./deploy/maistra-operator.yaml
```

By default, the operator watches for ServiceMeshControlPlane in all namespaces.  Typically, a cluster-wide control plane
is installed in `istio-system`.  For example:

```
$ oc apply -n istio-system -f ./deploy/examples/maistra_v1_servicemeshcontrolplane_cr_full.yaml
```

Example resources can be found in [./deploy/examples](./deploy/examples).

## Uninstall

If an existing ServiceMeshControlPlane cr has not been deleted, you need to delete the ServiceMeshControlPlane cr before deleting the istio operator. For example:

```
$ oc delete -n istio-system -f ./deploy/examples/maistra_v1_servicemeshcontrolplane_cr_full.yaml
```

### Uninstalling the Istio Operator

If you followed the instructions above for installation, the operator can be uninstalled by issuing the following delete
commands:

```
$ oc delete -n istio-operator -f ./deploy/maistra-operator.yaml
$ oc delete validatingwebhookconfiguration/istio-operator.servicemesh-resources.maistra.io
```

Once the Istio Operator has been uninstalled, the Jaeger Operator and the Kiali Operator should be uninstalled.

### Uninstalling the Jaeger Operator

To uninstall the Jaeger operator, execute the following commands:

```
oc delete -n observability -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/v1.13.1/deploy/operator.yaml
oc delete -n observability -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/v1.13.1/deploy/role_binding.yaml
oc delete -n observability -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/v1.13.1/deploy/role.yaml
oc delete -n observability -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/v1.13.1/deploy/service_account.yaml
oc delete -n observability -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/v1.13.1/deploy/crds/jaegertracing_v1_jaeger_crd.yaml
```

### Uninstalling the Elasticsearch Operator

To uninstall the Elasticsearch operator, execute the following commands:

```
oc delete -f https://raw.githubusercontent.com/openshift/elasticsearch-operator/release-4.1/manifests/05-deployment.yaml -n openshift-logging
oc delete -f https://raw.githubusercontent.com/openshift/elasticsearch-operator/release-4.1/manifests/04-crd.yaml -n openshift-logging
oc delete -f https://raw.githubusercontent.com/openshift/elasticsearch-operator/release-4.1/manifests/03-role-bindings.yaml
oc delete -f https://raw.githubusercontent.com/openshift/elasticsearch-operator/release-4.1/manifests/02-role.yaml
oc delete -f https://raw.githubusercontent.com/openshift/elasticsearch-operator/release-4.1/manifests/01-service-account.yaml -n openshift-logging
```

### Uninstalling the Kiali Operator

To uninstall the Kiali operator, execute the following command:

```
bash <(curl -L https://git.io/getLatestKialiOperator) --uninstall-mode true --operator-watch-namespace '**'
```

For more details on uninstalling the Kiali operator, see the [Kiali documentaton](https://www.kiali.io/documentation/getting-started/#_uninstall_kiali_operator_and_kiali).

## Multitenancy

The operator installs a control plane configured for multitenancy.  This installation reduces the scope of the control plane
to only those projects/namespaces listed in a `ServiceMeshMemberRoll`.  After installing the control plane, create/update
a ServiceMeshMemberRoll resource with the project/namespaces you wish to be part of the mesh.  The name of the
ServiceMeshMemberRoll resource must be named `default`.  The operator will configure the control plane to watch/manage pods
in those projects/namespaces and will configure the project/namespaces to work with the control plane.  (Note, auto-injection
only occurs after the project/namespace has become a member of the mesh.)

### ServiceMeshMemberRoll

A ServiceMeshMemberRoll is used to specify which projects/namespaces should be part of a service mesh installation.  It
has a single field in it's spec, which is a list of members, for example:

```yaml
apiVersion: maistra.io/v1
kind: ServiceMeshMemberRoll
metadata:
  # name must be default
  name: default
spec:
  members:
  # a list of projects/namespaces that should be joined into the service mesh
  # for example, the bookinfo project/namespace
  - bookinfo
```

## Customizing the Installation

The installation is easily customizable by modifying the `.spec.istio` section of the ServiceMeshControlPlane resource.  If you are
familiar with the Helm based installation, all of those settings are exposed through the operator.

The following sections describe common types of customizations.

### Custom Images

The image registry from which the Istio control plane images are pulled may be changed by adding a global `hub`
value to the ServiceMeshControlPlane specification.  The default `tag` used for the images may be changed in a similar fashion.
For example:

```yaml
apiVersion: maistra.io/v1
kind: ServiceMeshControlPlane
metadata:
  name: full-install
spec:
  istio:
    global:
      hub: my-private-registry.com/custom-namespace
      tag: 1.2.0-dev

  ...
```

### Image Pull Secrets

If access to the registry providing the Istio images is secure, you may add your access token settings.  This will add
`imagePullSecrets` to the appropriate ServiceAccount resources.  For example:

```yaml
apiVersion: maistra.io/v1
kind: ServiceMeshControlPlane
metadata:
  name: full-install
spec:
  istio:
    global:
      imagePullSecrets:
      - MyPullSecret
      - AnotherPullSecret

  ...
```

### Resource Limits

If you will be installing into an instance that is resource constrained, you most likely will need to modify the
resource requirements used by default.  Resource requirements can be set by default in `.spec.istio.global.defaultResources`.  For
example:

```yaml
apiVersion: maistra.io/v1
kind: ServiceMeshControlPlane
metadata:
  name: full-install
spec:
  istio:
    global:
      defaultResources:
        requests:
          cpu: 10m
          memory: 128Mi

  ...
```

### Component Customizations

Component specific customizations may be made by modifying the appropriate setting under the component key (e.g.
`.spec.tracing`).  Many of the global customizations described above may also be applied to specific components.
Some examples:

Customize resources (e.g. proxy, mixer):
```yaml
apiVersion: maistra.io/v1
kind: ServiceMeshControlPlane
metadata:
  name: full-install
spec:
  istio:
    global:
      proxy:
        resources:
          requests:
            cpu: 10m
            memory: 128Mi
    mixer:
      telemetry:
        resources:
          requests:
            cpu: 10m
            memory: 128Mi

  ...
```

Customize component image (e.g. Kiali):
```yaml
apiVersion: maistra.io/v1
kind: ServiceMeshControlPlane
metadata:
  name: full-install
spec:
  istio:
    kiali:
      hub: kiali
      tag: v1.0.0

  ...
```

## Developing the Istio Operator

You'll find instructions on how to build and run the Operator locally in [DEVEL.md](DEVEL.md). 

## Architecture

This operator provides a wrapper around the helm charts used when installing Istio via `helm template` or `helm install`.
As such, the custom resource used to define the features of the control plane maps directly to a `values.yaml` file, the
root of which is located in the resource's `.spec.istio` field.  See examples of a [minimal installation](./deploy/examples/maistra_v1_servicemeshcontrolplane_cr_minimal.yaml), [full installation](./deploy/examples/maistra_v1_servicemeshcontrolplane_cr_full.yaml), a [full installation with auth](./deploy/examples/maistra_v1_servicemeshcontrolplane_cr_auth.yaml).

## Modifications for Maistra

Aside from embedding all installation logic into the operator (e.g. removing `create-custom-resources.yaml` templates),
the the changes made to the base Istio charts can be found below.  For a specific list of all modifications, see
[patch-charts.sh](./tmp/build/patch-charts.sh).

### Component Modifications

#### General

* GODEBUG environment variable settings have been removed from all templates.
* A `maistra-version` label has been added to all resources.
* The `istio-multi` ServiceAccount and ClusterRoleBinding have been removed, as well as the `istio-reader` ClusterRole.
* All Ingress resources have been converted to OpenShift Route resources.

#### Galley
* A named `targetPort` has been added to the Galley Service.
* The Galley webhook port has been moved from 443 to 8443.
* The Galley health file has been moved to `/tmp/heath` (from `/health`)
* The `--validation-port` option has been added to the Galley.

#### Sidecar Injector

* Sidecar proxy init containers have been configured as privileged, regardless of `global.proxy.privileged` setting.
* The opt-out mechanism for injection has been modified when `sidecarInjectorWebhook.enableNamespacesByDefault` is enabled.
  Namespaces now opt-out by adding an `maistra.io/ignore-namespace` label to the namespace.
* A named `targetPort` has been added to the Sidecar Injector Service.
* The Sidecar Injector webhook port has been moved from 443 to 8443.

#### Gateways

* A Route has been added for the istio-ingressgateway gateway.
* The istio-egressgateway gateway has been enabled by default.

#### Prometheus

* The Prometheus init container has been modified to use the following image, `docker.io/prom/prometheus:v2.3.1`.

#### Grafana

* Has been enabled by default.
* Ingress has been enabled by default.
* A ServiceAccount has been added for Grafana.

#### Tracing

* Has been enabled by default.
* Ingress has been enabled by default.
* The `hub` value for the Jaeger images has changed to `jaegertracing` (from `docker.io/jaegertracing`).
* The tag used for the Jaeger images has been updated to `1.11`.
* The name for the Zipkin port name has changed to `jaeger-collector-zipkin` (from `http`)
* Jaeger uses Elasticsearch for storage.

#### Kiali

* Has been enabled by default.
* Ingress has been enabled by default.
* The `hub` value for the Kiali image has changed to `kiali` (from `docker.io/kiali`).
* The tag used for the Kiali image has been updated to `v1.0.0`.
* A Kiali CR is now created (for the Kiali Operator) as opposed to individual Kiali resources like ConfigMap, Deployment, etc.
* The auth strategy is "openshift".

## Known Issues

The following are known issues that need to be addressed:

* Istio CustomResourceDefinition resources are not removed during uninstall.
* Updates have not been tested (e.g. modifying a ServiceMeshControlPlane resource to enable/disable a component).
* Uninstall is a little sloppy (i.e. resources are just deleted, and not in an intelligent fashion).
* Reconciliation is only performed on the ServiceMeshControlPlane resource (i.e. the operator is not watching installed resources,
  e.g. galley Deployment).  This means users may modify those resources and the operator will not revert them (unless
  the ServiceMeshControlPlane resource is modified).
* Rollout may hang if configuration changes made to the istio-operator deployment.  (I believe this has to do with
  leader election, where the new deployment fails to become ready until the prior one is terminated.)
