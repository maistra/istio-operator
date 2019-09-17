# Developing the Maistra Istio Operator

## Building the binary and the container image

To build the Operator container image, run:
```bash
IMAGE=docker.io/<your username>/istio-operator:latest tmp/build/docker_build.sh
```

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

