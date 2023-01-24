# Monitoring solutions for OpenShift Service Mesh

### Prepare users and permissions

1. Prepare htpasswd identity provider
```shell
oc login -u kubeadmin https://api.crc.testing:6443
touch htpasswd
htpasswd -Bb htpasswd clusteradmin clusteradm
htpasswd -Bb htpasswd meshadmin-1 adm1
htpasswd -Bb htpasswd developer-1 dev1
htpasswd -Bb htpasswd meshadmin-2 adm2
oc delete secret htpass-secret -n openshift-config
oc create secret generic htpass-secret --from-file=htpasswd=htpasswd -n openshift-config
oc apply -f oauth.yaml
```

2. Configure roles for users:
```shell
oc create user clusteradmin
oc create identity simple-htpasswd:clusteradmin
oc create useridentitymapping simple-htpasswd:clusteradmin clusteradmin
oc adm policy add-cluster-role-to-user cluster-admin clusteradmin

for i in 1 2
do
  oc new-project istio-system-$i
  oc new-project bookinfo-$i
  oc create user meshadmin-$i
  oc create identity simple-htpasswd:meshadmin-$i
  oc create useridentitymapping simple-htpasswd:meshadmin-$i meshadmin-$i
  oc adm policy add-role-to-user admin meshadmin-$i -n istio-system-$i
  oc adm policy add-role-to-user admin meshadmin-$i -n bookinfo-$i
  sed "s/{{username}}/meshadmin-$i/g" rbac/monitors.yaml | oc apply -n istio-system-$i -f -
  sed "s/{{username}}/meshadmin-$i/g" rbac/monitors.yaml | oc apply -n bookinfo-$i -f -
  sed "s/{{username}}/meshadmin-$i/g" rbac/telemetry.yaml | oc apply -n istio-system-$i -f -
done

oc create user developer-1
oc create identity simple-htpasswd:developer-1
oc create useridentitymapping simple-htpasswd:developer-1 developer-1
oc adm policy add-role-to-user edit developer-1 -n bookinfo-1
```

3. Wait until you can log in as `clusteradmin` (it may take a few minutes):
```shell
oc login -u clusteradmin https://api.crc.testing:6443
```

4. Delete default users (optional step):
```shell
oc delete user developer
oc delete user kubeadmin
oc delete identity developer:developer
oc delete identity developer:kubeadmin
```

### Service Mesh and Monitoring integration

1. Enable user-workload monitoring:
```shell
oc login -u clusteradmin https://api.crc.testing:6443
oc apply -f openshift-monitoring/enable-monitoring-in-user-workloads.yaml
```

3. Install OpenShift Service Mesh operator.

4. Deploy control plane and an app for the first tenant:
```shell
oc login -u meshadmin-1 https://api.crc.testing:6443
oc apply -n istio-system-1 -f openshift-monitoring/mesh.yaml
sed 's/{{host}}/bookinfo-1/g' route.yaml | oc apply -n istio-system-1 -f -
```
Wait until istiod is ready and then apply:
```shell
oc apply -n istio-system-1 -f telemetry.yaml
oc apply -n bookinfo-1 -f openshift-monitoring/bookinfo.yaml
oc apply -n bookinfo-1 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.3/samples/bookinfo/networking/bookinfo-gateway.yaml
```

TODO: Gateway injection does not work in this setup. Try with 2.3.1:
```shell
sed 's/{{host}}/httpbin-1/g' gateway-injection.yaml | oc apply -n httpbin-1 -f -
```

5. Generate traffic:
```shell
while true; do curl -v bookinfo-1.apps-crc.testing:80/productpage > /dev/null; sleep 1; done
```

6. Configure monitoring to scrape merged metrics:
```shell
oc apply -n istio-system-1 -f openshift-monitoring/istiod-monitor.yaml
oc apply -n istio-system-1 -f openshift-monitoring/istio-proxies-monitor.yaml
oc apply -n bookinfo-1 -f openshift-monitoring/istio-proxies-monitor.yaml
```

8. Deploy Kiali as `clusteradmin`:
```shell
oc login -u clusteradmin https://api.crc.testing:6443
SECRET=`oc get secret -n openshift-user-workload-monitoring | grep  prometheus-user-workload-token | head -n 1 | awk '{print $1 }'`
TOKEN=`echo $(oc get secret $SECRET -n openshift-user-workload-monitoring -o json | jq -r '.data.token') | base64 -d`
sed "s/{{token}}/$TOKEN/g" openshift-monitoring/kiali.yaml | oc apply -n istio-system-1 -f -
```

## Custom Prometheus Operator

1. Login as `clusteradmin` and install Prometheus Operator in the OCP console:
```shell
oc login -u clusteradmin https://api.crc.testing:6443
oc new-project custom-prometheus
```

2. Exclude second mesh from OpenShift Monitoring:
```shell
oc label namespace custom-prometheus 'openshift.io/user-monitoring=false'
oc label namespace istio-system-2 'openshift.io/user-monitoring=false'
oc label namespace bookinfo-2 'openshift.io/user-monitoring=false'
```

3. Grant `meshadmin-2` permissions to create and configure Prometheus:
```shell
oc adm policy add-role-to-user admin meshadmin-2 -n custom-prometheus
oc apply -f custom-prometheus/allow-to-manage-prometheus.yaml
oc apply -n custom-prometheus -f custom-prometheus/custom-prometheus-permissions.yaml
oc apply -n istio-system-2 -f custom-prometheus/custom-prometheus-permissions.yaml
oc apply -n bookinfo-2 -f custom-prometheus/custom-prometheus-permissions.yaml
sed "s/{{username}}/meshadmin-2/g" rbac/monitors.yaml | oc apply -n custom-prometheus -f -
```

3. Login as `meshadmin-2` and deploy `Prometheus`:
```shell
oc login -u meshadmin-2 https://api.crc.testing:6443
```

4. Deploy SMCP:
```shell
oc apply -n istio-system-2 -f custom-prometheus/mesh.yaml
sed 's/{{host}}/bookinfo-2/g' route.yaml | oc apply -n istio-system-2 -f -
```

Wait until istiod is ready and then apply:
```shell
oc apply -f custom-prometheus/prometheus.yaml
oc apply -n istio-system-2 -f telemetry.yaml
oc apply -n bookinfo-2 -f custom-prometheus/bookinfo.yaml
oc apply -n bookinfo-2 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.3/samples/bookinfo/networking/bookinfo-gateway.yaml
```

5. Enable monitoring:
```shell
oc apply -f custom-prometheus/istiod-monitor.yaml
oc apply -f custom-prometheus/istio-proxies-monitor.yaml
oc apply -f custom-prometheus/app-mtls-monitor.yaml
```

6. Generate traffic:
```shell
while true; do curl -v bookinfo-2.apps-crc.testing:80/productpage > /dev/null; sleep 1; done
```

### Issues

1. SMCP must provide a way to enable `telemetry.v2.prometheus.enabled` without deploying Prometheus.
The settings below don't work, because `telemetry.v2.prometheus.enabled` is set to `false` when `spec.addons.prometheus.enabled` is `false`.
```yaml
  techPreview:
    meshConfig:
      enablePrometheusMerge: true
    telemetry:
      enabled: true
      v2:
        prometheus:
          enabled: true
```
The workaround for this problem is enabling `extensionProviders`, but this should be GA:
```yaml
  techPreview:
    meshConfig:
      extensionProviders:
      - name: prometheus
        prometheus: {}
```
2. In the examples, I configured Kiali to use cluster-wide Thanos token, but it should use a namespace-scoped token.
   A potential solution could be using user's token from OAuth proxy, but I don't know if this token is available for Kiali after login.  
   TODO:
   - verify what happens when a user queries Thanos for multiple namespaces where one of namespaces does not belong to that user;
   - verify why Kiali restricts access when `developer-1` log in to Kiali, while it uses cluster-wide token;
   - verify why Thanos returns 403 when a Kiali uses its own token: `spec.external_services.prometheus.auth.use_kiali_token: true`.
   - how to set proper permissions for kiali-service-account to permit Kiali querying Thanos with its own token.

```shell
kubectl port-forward service/thanos-querier -n openshift-monitoring 9091:9091
curl -X GET -vkG "https://localhost:9091/api/v1/query?" --data-urlencode "query=up{namespace='istio-system-1'}" -H "Authorization: Bearer <my-token>"
```

3. Kiali could work without cluster-wide token and could just use its own token if it supports [prom-label-proxy](https://github.com/prometheus-community/prom-label-proxy) API.
   This API slightly differs from the current API and only requires adding requested namespace to the URL as below: 
```shell
curl -X GET -vkG "https://localhost:9092/api/v1/query?up&namespace=bookinfo-1&" --data-urlencode "query=up" -H "Authorization: Bearer $KIALI_TOKEN"
```

To permit Kiali to access that API, Kiali Operator would have to add the following role to the kiali-service-account:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kiali-monitoring-rbac
rules:
- apiGroups: ["metrics.k8s.io"]
 resources:
 - pods
 verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kiali-monitoring-rbac
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kiali-monitoring-rbac
subjects:
- kind: ServiceAccount
  name: kiali-service-account
  namespace: istio-system-1
```
This is necessary, because Thanos Querier checks this role on port `tenancy` (9092) -
this is configured [here](https://github.com/openshift/cluster-monitoring-operator/blob/2b4844db3e64a6764b702171f76ee5eb32145e3f/assets/thanos-querier/kube-rbac-proxy-secret.yaml).
Then Prometheus could be configured without any additional token.
```yaml
    prometheus:
      auth:
        type: bearer
        use_kiali_token: true
      thanos_proxy:
        enabled: true
      url: https://thanos-querier.openshift-monitoring.svc.cluster.local:9092
```
