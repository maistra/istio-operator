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
bash <(curl -L https://kiali.io/getLatestKialiOperator) --operator-image-version v1.0 --operator-watch-namespace '**' --accessible-namespaces '**' --operator-install-kiali false
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
bash <(curl -L https://kiali.io/getLatestKialiOperator) --uninstall-mode true --operator-watch-namespace '**'
```

For more details on uninstalling the Kiali operator, see the [Kiali documentaton](https://www.kiali.io/documentation/getting-started/#_uninstall_kiali_operator_and_kiali).

# Developing the Maistra Istio Operator

## Resources

All non-compiled resources that are included in the operator image are stored in `/resources`  These include the helm charts and the SMCP templates (i.e. profiles).  Resources for each version of Maistra supported by the operator are stored in individual subdirectories (e.g. v2.0, v2.1, etc.).  Earlier versions are copied in from older branches of the operator.  Only resources for the current version of the operator are maintained within this repository.

### Helm Charts (/resources/helm)

This directory consists of a directory containing overlays for the charts (`./overlays`) and a directory for each version of charts supported by this operator.  Earlier versions are copies from other branches of the operator (e.g. v2.0 copies from maistra-2.0/resources/helm/v2.0).  This allows chart updates for older versions to be accomplished by updating the older source branch.  It also allows chart customization to be confined to the latest version (i.e. as the repository evolves, it doesn't have to maintain the chart patching code for older versions).

The current version of the charts can be regenerated by running `make generate-charts`.

### SMCP Templates (/resources/smcp-templates)

This dierectory contains the SMCP templates for various versions of Maistra supported by this operator.  As with the other resources, earlier versions of are copies from other branches, with only this operator's version being maintained in this branch.

### OLM Manifests

OLM manifests are generated from the resources in the `/deploy` folder.  This helps ensure that the same resources (e.g. Deployment, ClusterRole, etc.) used for developer installs is used in the OLM manifests.  To regenerate the manifests after an updating these files run `make generate-manifests`.

## Building the binary and the container image

To build the Operator container image, run:
```bash
IMAGE=docker.io/<your username>/istio-operator:latest make image
```

If you do not specify `IMAGE` it will use `docker.io/maistra/istio-ubi8-operator:${MAISTRA_VERSION}` as the default.

This builds the binary, downloads and patches the Istio Helm charts, and finally builds the container image. The build script doesn't push the image to the container registry, so you'll need to push it manually:
```bash
docker push docker.io/<your username>/istio-operator:latest
```

## Running the operator locally

During development, you can run Istio Operator locally on your dev machine and have it reconcile objects in a local or remote OpenShift cluster. You can run the Operator from within your IDE and even use a debugger.

To run the Operator locally, you need to set the following environment variables:

- `KUBECONFIG=<path to your kubectl config file>`
- `POD_NAME=istio-operator`
- `POD_NAMESPACE=istio-operator`
- `ISTIO_CNI_IMAGE=some-registry/istio-cni-image:latest`
- `ISTIO_CNI_IMAGE_PULL_SECRET=my-pull-secret` (required if the registry hosting the CNI image is not public; secret must be in operator's namespace as defined above)

Make sure you run `tmp/build/docker_build.sh` before running the Operator to generate the Helm charts and templates in `tmp/_output/resources`.

Before running the Operator, don't forget to stop the Operator running in the cluster:
```bash
oc -n istio-operator scale deployment istio-operator --replicas 0
```  

Run the operator locally with the following command-line options:
```bash
--resourceDir tmp/_output/resources/
```

### Modifying the ValidatingWebhookConfiguration 

Because the Operator installs a `ValidatingWebhookConfiguration` that points to the Operator pod running inside the cluster, all CRUD operations on `ServiceMeshControlPlane` or `ServiceMeshMemberRoll` objects will fail. To fix this, you either need to remove the `ValidatingWebhookConfiguration`:
```bash
oc delete validatingwebhookconfigurations istio-operator.servicemesh-resources.maistra.io 
```

or point it to the Operator process running on your local machine by patching the `Service` and `Endpoints` objects:
```bash
oc -n istio-operator patch svc admission-controller --type=json -p='[{"op": "remove", "path": "/spec/selector"}]'
oc -n istio-operator patch ep admission-controller --type=json -p='[{"op": "add","path": "/subsets","value": [{"addresses": [{"ip": "192.168.1.100"}],"ports": [{"port": 11999}]}]}]'
```

Replace `192.168.1.100` with the IP of your dev machine, where the Operator is running. You also need to ensure that the OpenShift master node can connect to your dev machine on port 11999. Note: this is trivial if running OpenShift through DIND.

 
NOTE: you'll need to remove the `ValidatingWebhookConfiguration` or patch the `Service` and `Endpoints` every time after you restart the Operator, as it resets them during startup.

   
 ## Configuring development logging

By default, the Istio Operator log output is in JSON format, which is meant to be consumed by a centralized logging system, not read by humans directly, due to its hard-to-read format:
```
{"level":"info","ts":1565598764.4637527,"logger":"cmd","caller":"manager/main.go:58","msg":"Starting Istio Operator version.BuildInfo{Version:\"unknown\", GitRevision:\"unknown\", BuildStatus:\"unknown\", GitTag:\"unknown\", GoVersion:\"go1.12.5\", GoArch:\"linux/amd64\", OperatorSDK:\"v0.2.1\"}"}
{"level":"info","ts":1565598764.4639816,"logger":"cmd","caller":"manager/main.go:62","msg":"watching all namespaces"}
{"level":"info","ts":1565598764.4676018,"logger":"leader","caller":"leader/leader.go:55","msg":"Trying to become the leader."}
```

During development you can switch to using plain text by running the Operator with the following option:
```bash
--logConfig devel
``` 

This produces the following log format, which is much easier to read:
```
2019-08-12T10:31:20.051+0200	INFO	cmd	manager/main.go:58	Starting Istio Operator version.BuildInfo{Version:"unknown", GitRevision:"unknown", BuildStatus:"unknown", GitTag:"unknown", GoVersion:"go1.12.5", GoArch:"linux/amd64", OperatorSDK:"v0.2.1"}
2019-08-12T10:31:20.051+0200	INFO	cmd	manager/main.go:62	watching all namespaces
2019-08-12T10:31:20.057+0200	INFO	leader	leader/leader.go:55	Trying to become the leader.
```

## Updating the Operator Version

Here are the steps required to seed a new version of the operator:

1. Rename the directory corresponding to the latest version of each resource type to the new version (e.g. from v1.1 to v1.2).
2. Update the Makefile to add a target to copy the resources from the previous version (e.g. copy github.com/maistra/istio-operator[maistra-1.1]/resources/helm/v1.1 to /resources/helm/v1.1).  This should be done for the charts and the smcp templates.
3. Update the Makefile to add targets for collecting the resources for the new versions (e.g. collect-1.2-charts, collect-1.2-templates).
4. Update `MAISTRA_VERSION` in the Makefile to the new version.
