# Monitoring solutions for OpenShift Service Mesh

### Prerequisites
1. OpenShift Service Mesh Operator 2.4+.
2. Kiali Operator 1.65+.
3. OpenShift user-workload monitoring stack is available (`crc config set enable-cluster-monitoring true`).

### Prepare users and permissions to simulate multi-tenant environment

1. Prepare htpasswd identity provider
```shell
oc login -u kubeadmin https://api.crc.testing:6443
touch htpasswd
htpasswd -Bb htpasswd clusteradmin clusteradm
htpasswd -Bb htpasswd meshadmin-1 adm1
htpasswd -Bb htpasswd developer-1 dev1
htpasswd -Bb htpasswd meshadmin-2 adm2
htpasswd -Bb htpasswd meshadmin-3 adm3
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

for i in 1 2 3
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

## OpenShift Monitoring stack

1. Enable user-workload monitoring:
```shell
oc login -u clusteradmin https://api.crc.testing:6443
oc apply -f openshift-monitoring/enable-monitoring-in-user-workloads.yaml
```

2. Wait until UWM workloads are ready and create secret for Kiali as `clusteradmin`:
```shell
SECRET=`oc get secret -n openshift-user-workload-monitoring | grep  prometheus-user-workload-token | head -n 1 | awk '{print $1 }'`
TOKEN=`echo $(oc get secret $SECRET -n openshift-user-workload-monitoring -o json | jq -r '.data.token') | base64 -d`
oc create secret generic thanos-querier-web-token -n istio-system-1 --from-literal=token=$TOKEN
```

3. Deploy Kiali, Istio and bookinfo app for the first tenant:
```shell
oc login -u meshadmin-1 https://api.crc.testing:6443
oc apply -n istio-system-1 -f openshift-monitoring/kiali.yaml
oc apply -n istio-system-1 -f openshift-monitoring/mesh.yaml
```

4. Wait until istiod is ready and then apply:
```shell
oc apply -n istio-system-1 -f telemetry.yaml
oc apply -n bookinfo-1 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.4/samples/bookinfo/networking/bookinfo-gateway.yaml
oc apply -n bookinfo-1 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.4/samples/bookinfo/platform/kube/bookinfo.yaml
oc patch -n bookinfo-1 deployment productpage-v1 -p '{"spec":{"template":{"spec":{"containers":[{"name": "productpage", "image":"quay.io/jewertow/productpage:metrics"}]}}}}'
oc patch -n bookinfo-1 deployment productpage-v1 -p '{"spec": {"template":{"metadata":{"annotations":{"prometheus.io/scrape":"true","prometheus.io/port":"9080","prometheus.io/path":"/metrics"}}}}}'
```

5. Generate traffic:
```shell
ISTIO_SYSTEM_1_INGRESS_HOST=$(oc get routes -n istio-system-1 istio-ingressgateway -o jsonpath='{.spec.host}')
while true; do curl -v "http://$ISTIO_SYSTEM_1_INGRESS_HOST:80/productpage" > /dev/null; sleep 1; done
```

6. Configure monitoring to scrape merged metrics:
```shell
oc apply -n istio-system-1 -f openshift-monitoring/istiod-monitor.yaml
oc apply -n istio-system-1 -f openshift-monitoring/istio-proxies-monitor.yaml
oc apply -n bookinfo-1 -f openshift-monitoring/istio-proxies-monitor.yaml
```

7. Go to the "Observe" view and display istio-proxy and application metrics:

![istio-proxy metrics](openshift-monitoring/screenshots/istio-proxy-metrics.png)
![application-metrics](openshift-monitoring/screenshots/application-metrics.png)

8. Go to Kiali dashboard and verify that users can see metrics only from the projects to which they belong:

![kiali-admin-view](openshift-monitoring/screenshots/kiali-project-admin-view.png)
![kiali-developer](openshift-monitoring/screenshots/kiali-developer-view.png)

## Custom Prometheus Operator

1. Login as `clusteradmin` and install Prometheus Operator through the operator hub:
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

3. Login as `meshadmin-2` and deploy service mesh:
```shell
oc login -u meshadmin-2 https://api.crc.testing:6443
oc apply -n istio-system-2 -f custom-prometheus/kiali.yaml
oc apply -n istio-system-2 -f custom-prometheus/mesh.yaml
```

Wait until istiod is ready and then apply:
```shell
oc apply -f custom-prometheus/prometheus.yaml
oc apply -n istio-system-2 -f telemetry.yaml
oc apply -n bookinfo-2 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.4/samples/bookinfo/networking/bookinfo-gateway.yaml
oc apply -n bookinfo-2 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.4/samples/bookinfo/platform/kube/bookinfo.yaml
oc patch -n bookinfo-2 deployment productpage-v1 -p '{"spec":{"template":{"spec":{"containers":[{"name": "productpage", "image":"quay.io/jewertow/productpage:metrics"}]}}}}'
```

5. Enable monitoring:
```shell
oc apply -f custom-prometheus/istiod-monitor.yaml
oc apply -f custom-prometheus/istio-proxies-monitor.yaml
oc apply -f custom-prometheus/app-mtls-monitor.yaml
```

6. Generate traffic:
```shell
ISTIO_SYSTEM_2_INGRESS_HOST=$(oc get routes -n istio-system-2 istio-ingressgateway -o jsonpath='{.spec.host}')
while true; do curl -v "http://$ISTIO_SYSTEM_2_INGRESS_HOST:80/productpage" > /dev/null; sleep 1; done
```

7. Expose prometheus dashboard to localhost and verify that istio-proxy and application metrics are scraped:
```shell
kubectl port-forward service/prometheus-operated -n custom-prometheus 9090:9090
```
![prometheus-targets](custom-prometheus/screenshots/prometheus-targets.png)

8. Go to Kiali dashboard and verify that Prometheus scrapes application metrics over mTLS:

![kiali-prometheus-mtls](custom-prometheus/screenshots/kiali-prometheus-mtls.png)

## Federating metrics from OSSM to OpenShift Monitoring

1. Enable user-workload monitoring:
```shell
oc login -u clusteradmin https://api.crc.testing:6443
oc apply -f openshift-monitoring/enable-monitoring-in-user-workloads.yaml
```

2. Install OpenShift Service Mesh operator.

3. Deploy control plane and an app for the first tenant:
```shell
oc login -u meshadmin-3 https://api.crc.testing:6443
oc apply -n istio-system-3 -f federation/mesh.yaml
sed 's/{{host}}/bookinfo-3/g' route.yaml | oc apply -n istio-system-3 -f -
```
Wait until istiod is ready and then apply:
```shell
oc apply -n bookinfo-3 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.4/samples/bookinfo/platform/kube/bookinfo.yaml
oc apply -n bookinfo-3 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.4/samples/bookinfo/networking/bookinfo-gateway.yaml
```

4. Generate traffic:
```shell
ISTIO_SYSTEM_3_INGRESS_HOST=$(oc get routes -n istio-system-3 istio-ingressgateway -o jsonpath='{.spec.host}')
while true; do curl -v "http://$ISTIO_SYSTEM_3_INGRESS_HOST:80/productpage" > /dev/null; sleep 1; done
```

5. Federate metrics from Service Mesh to Monitoring:
```shell
oc apply -n istio-system-3 -f federation/federation-service-monitor.yaml
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
curl -X GET -vkG "https://localhost:9091/api/v1/query?" --data-urlencode "query=up{namespace='istio-system-1'}" -H "Authorization: Bearer $TOKEN"
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
- apiGroups: ["metrics.k8s.io/v1beta1"]
  resources: ["pods"]
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

#### RBAC proxy in Thanos Querier does not work as expected
How to reproduce:
1. Grant kiali-service-account role to get metrics for pods in namespace `istio-system-2`:
```shell
oc apply -n istio-system-2 -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kiali-monitoring-rbac
rules:
- apiGroups: ["metrics.k8s.io/v1beta1"]
  resources: ["pods"]
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
EOF 
```
2. Deploy Kiali:
```shell
oc login -u meshadmin-1 https://api.crc.testing:6443
oc apply -n istio-system-1 -f openshift-monitoring/kiali.yaml
```
3. Get Kiali token:
```shell
KIALI_TOKEN=$(oc exec -n istio-system-1 $(oc get pods -n istio-system-1 -l app=kiali -o jsonpath='{.items[].metadata.name}') -- cat /var/run/secrets/kubernetes.io/serviceaccount/token)
```
4. Expose Thanos Querier tenancy port:
```shell
oc port-forward service/thanos-querier -n openshift-monitoring 9092:9092
```
5. Get metrics for namespace "istio-system-2" from Thanos Querier on the `tenancy` port:
```shell
curl -X GET -vkG "https://localhost:9092/api/v1/query?namespace=istio-system-2" --data-urlencode "query=up" -H "Authorization: Bearer $KIALI_TOKEN"
```
Output:
```
Note: Unnecessary use of -X or --request, GET is already inferred.
*   Trying 127.0.0.1:9092...
* Connected to localhost (127.0.0.1) port 9092 (#0)
* ALPN, offering h2
* ALPN, offering http/1.1
* successfully set certificate verify locations:
*  CAfile: /etc/pki/tls/certs/ca-bundle.crt
*  CApath: none
* TLSv1.3 (OUT), TLS handshake, Client hello (1):
* TLSv1.3 (IN), TLS handshake, Server hello (2):
* TLSv1.3 (IN), TLS handshake, Encrypted Extensions (8):
* TLSv1.3 (IN), TLS handshake, Request CERT (13):
* TLSv1.3 (IN), TLS handshake, Certificate (11):
* TLSv1.3 (IN), TLS handshake, CERT verify (15):
* TLSv1.3 (IN), TLS handshake, Finished (20):
* TLSv1.3 (OUT), TLS change cipher, Change cipher spec (1):
* TLSv1.3 (OUT), TLS handshake, Certificate (11):
* TLSv1.3 (OUT), TLS handshake, Finished (20):
* SSL connection using TLSv1.3 / TLS_AES_128_GCM_SHA256
* ALPN, server accepted to use h2
* Server certificate:
*  subject: CN=thanos-querier.openshift-monitoring.svc
*  start date: Jan 26 18:35:42 2023 GMT
*  expire date: Jan 25 18:35:43 2025 GMT
*  issuer: CN=openshift-service-serving-signer@1662651732
*  SSL certificate verify result: self signed certificate in certificate chain (19), continuing anyway.
* Using HTTP2, server supports multiplexing
* Connection state changed (HTTP/2 confirmed)
* Copying HTTP/2 data in stream buffer to connection buffer after upgrade: len=0
* Using Stream ID: 1 (easy handle 0x55bace7f0e00)
> GET /api/v1/query?namespace=istio-system-2&query=up HTTP/2
> Host: localhost:9092
> user-agent: curl/7.79.1
> accept: */*
> authorization: Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6Ik9rTnkzdnF1Q1V2MWdQb0ZwdUlqMFF5SXl0b2FkRjdLM1dzZ0Jpb0M3LTQifQ.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjIl0sImV4cCI6MTcwNjQ3NzA4MCwiaWF0IjoxNjc0OTQxMDgwLCJpc3MiOiJodHRwczovL2t1YmVybmV0ZXMuZGVmYXVsdC5zdmMiLCJrdWJlcm5ldGVzLmlvIjp7Im5hbWVzcGFjZSI6ImlzdGlvLXN5c3RlbS0xIiwicG9kIjp7Im5hbWUiOiJraWFsaS04NGM3NDY1ZDg1LW14Y2w0IiwidWlkIjoiMmIwMzVkNWUtMGZjOS00ZGFhLWJmMmYtMzM0YmJmMGQ4MjczIn0sInNlcnZpY2VhY2NvdW50Ijp7Im5hbWUiOiJraWFsaS1zZXJ2aWNlLWFjY291bnQiLCJ1aWQiOiI2NTNlYjFkMi1iMGVjLTQ0NGQtYmQ4Yy1mZWJiNDE0MTc3YTkifSwid2FybmFmdGVyIjoxNjc0OTQ0Njg3fSwibmJmIjoxNjc0OTQxMDgwLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6aXN0aW8tc3lzdGVtLTE6a2lhbGktc2VydmljZS1hY2NvdW50In0.CLzwtaUrZ7PZzXsOLRk5HTAuFcYli3L4BezUfQngDez2XnXbU3lgsrTMg7PGH1fGkZI639KvqIvSzFPk-DIXzyRbuR9LCG-lraXd5qWB8TAPLh3SU6pf5jVN8nLGmKWtCFUTKU-AzkJfAmXjcjXBixNqMUwFfFasDkI_3bjUCz13veKvoLW_PeVRDmZateLPx7Z_8vvwn9FQEQKU4P1ecSyq_C8wUuXyi3DUgLBM1ZEFXIDF-w_ihZgvLfu-5jZbhDH26eMxHH_MybPHSkb9-IAa0J0VYsYnA6OJfj0_oCZZS_Xj8tgPwqybGcBM1J1IzjHtFAWAubhU196p9woW7xXeYmzfn47lf5B_0KqNuMT_3U3C-GWKkqVduQoh4MKw93mcNkhpqNnkscx9vndisM_gp248XKD0vCn5H0wh-N3xEDvnmCvHbtlbs7BmLMdGk0zxTgAtAZLC32mKv70mFCvDdamnOWW9_nXBAhEThtlzkku1Gyb3jB4UprYNbZVw3VznZ4UkvpEFbY-3_dOtdYaJpBAHeAAWSRX-smCvf0GvZHgQHlw2G2z_8bgi5rnCRGfujHC03SEBGjia4yF54Fpwef6b_gw242-FvczlRun1ArakYHPYZtPCJidJNDp8zCB1enY1_aIXcIGK0q7rWMV30MszeaDmoW6gp56BPlw
> 
* TLSv1.3 (IN), TLS handshake, Newsession Ticket (4):
* Connection state changed (MAX_CONCURRENT_STREAMS == 250)!
< HTTP/2 403 
< content-type: text/plain; charset=utf-8
< x-content-type-options: nosniff
< content-length: 115
< date: Sat, 28 Jan 2023 21:37:53 GMT
< 
Forbidden (user=system:serviceaccount:istio-system-1:kiali-service-account, verb=get, resource=pods, subresource=)
* Connection #0 to host localhost left intact
```

Configuration of Thanos Querier RBAC proxy can be found [here](https://github.com/openshift/cluster-monitoring-operator/blob/release-4.11/assets/thanos-querier/kube-rbac-proxy-secret.yaml).
