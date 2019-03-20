# Maistra Istio Operator

This project is an operator (controller) that can be used to manage the installation of an [Istio](https://istio.io) control plane.

## Installation

All resource definitions required to install the operator can be found in the [deploy](./deploy) directory, and can be
installed easily using your favorite Kubernetes command-line client.  You must have `cluster-admin` privileges to install
the operator.

```
$ oc apply -n istio-operator -f ./deploy/
```

or

```
$ kubectl apply -n istio-operator -f ./deploy/
```

By default, the operator watches for ControlPlane or Installation resources in the `istio-system` namespace, which is
where the control plane will be installed.  For example:

```
$ oc apply -n istio-system ./deploy/examples/istio_v1alpha3_controlplane_cr_basic.yaml
```

Example resources can be found in [./deploy/examples](./deploy/examples).

## Uninstall

If you followed the instructions above for installation, the operator can be uninstalled by simply issuing a delete
command against the same resources.  For example:

```
$ oc delete -n istio-operator -f ./deploy/
```

## Customizing the Installation

The installation is easily customizable by modifying the `.spec.istio` section of the ControlPlane resource.  If you are
familiar with the Helm based installation, all of those settings are exposed through the operator.

The following sections describe common types of customizations.

### Custom Images

The image registry from which the Istio control plane images are pulled may be changed by adding a global `hub`
value to the ControlPlane specification.  The default `tag` used for the images may be changed in a similar fashion.
For example:

```yaml
apiVersion: istio.openshift.com/v1alpha3
kind: ControlPlane
metadata:
  name: basic-install
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
apiVersion: istio.openshift.com/v1alpha3
kind: ControlPlane
metadata:
  name: basic-install
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
resource limits used by default.  Resource limits can be set by default in `.spec.istio.global.defaultResources`.  For
example:

```yaml
apiVersion: istio.openshift.com/v1alpha3
kind: ControlPlane
metadata:
  name: basic-install
spec:
  istio:
    global:
      defaultResources:
        requests:
          cpu: 10m
          memory: 128Mi
        limits:
          cpu: 100m
          memory: 256Mi

  ...
```

### Component Customizations

Component specific customizations may be made by modifying the appropriate setting under the component key (e.g.
`.spec.tracing`).  Many of the global customizations described above may also be applied to specific components.
Some examples:

Customize resources (e.g. proxy, mixer):
```yaml
apiVersion: istio.openshift.com/v1alpha3
kind: ControlPlane
metadata:
  name: basic-install
spec:
  istio:
    global:
      proxy:
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 128Mi
    mixer:
      telemetry:
        resources:
          requests:
            cpu: 100m
            memory: 1G
          limits:
            cpu: 500m
            memory: 4G

  ...
```

Customize component image (e.g. Kiali):
```yaml
apiVersion: istio.openshift.com/v1alpha3
kind: ControlPlane
metadata:
  name: basic-install
spec:
  istio:
    kiali:
      hub: kiali
      tag: v0.15.0

  ...
```


## Architecture

This operator provides a wrapper around the helm charts used when installing Istio via `helm template` or `helm install`.
As such, the custom resource used to define the features of the control plane maps directly to a `values.yaml` file, the
root of which is located in the resource's `.spec.istio` field.  See examples of a [basic installation](./deploy/examples/istio_v1alpha3_controlplane_cr_basic.yaml)
or a [secured installation](./deploy/examples/istio_v1alpha3_controlplane_cr_secure.yaml).

## Modifications for Maistra

Aside from embedding all installation logic into the operator (e.g. removing `create-custom-resources.yaml` templates),
the the changes made to the base Istio charts can be found below.  For a specific list of all modifications, see
[download-charts.sh](./tmp/build/download-charts.sh).

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
  Namespaces now opt-out by adding an `istio.openshift.com/ignore-namespace` label to the namespace.
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
* The tag used for the Kiali image has been updated to `v0.15.0`.
* Updates have been made to the Kiali ConfigMap.
* Updates have been made to the ClusterRole settings for Kiali.
* A demo Secret has been added that will get created using values from `kiali.dashboard.user` and `kiali.dashboard.passphrase`,
  if specified.

## Known Issues

The following are known issues that need to be addressed:

* Istio CustomResourceDefinition resources are not removed during uninstall.
* Updates have not been tested (e.g. modifying a ControlPlane resource to enable/disable a component).
* Uninstall is a little sloppy (i.e. resources are just deleted, and not in an intelligent fashion).
* Reconciliation is only performed on the ControlPlane resource (i.e. the operator is not watching installed resources,
  e.g. galley Deployment).  This means users may modify those resources and the operator will not revert them (unless
  the ControlPlane resource is modified).
* Rollout may hang if configuration changes made to the istio-operator deployment.  (I believe this has to do with
  leader election, where the new deployment fails to become ready until the prior one is terminated.)