# Monitoring solutions for OpenShift Service Mesh

### Prepare users and permissions

1. Prepare htpasswd identity provider
```shell
oc login -u kubeadmin https://api.crc.testing:6443
touch htpasswd
htpasswd -Bb htpasswd clusteradmin clusteradm
htpasswd -Bb htpasswd meshadmin-1 adm1
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

for i in 1 2
do
  oc new-project istio-system-$i
  oc new-project bookinfo-$i
  oc create user meshadmin-$i
  oc create identity simple-htpasswd:meshadmin-$i
  oc create useridentitymapping simple-htpasswd:meshadmin-$i meshadmin-$i
  oc adm policy add-role-to-user admin meshadmin-$i -n istio-system-$i
  oc adm policy add-role-to-user admin meshadmin-$i -n bookinfo-$i
  sed "s/{{username}}/meshadmin-$i/g" allow-admin-to-manage-telemetry-and-monitors.yaml | oc apply -n istio-system-$i -f -
  sed "s/{{username}}/meshadmin-$i/g" allow-admin-to-manage-telemetry-and-monitors.yaml | oc apply -n bookinfo-$i -f -
done
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
oc apply -n bookinfo-1 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.3/samples/bookinfo/platform/kube/bookinfo.yaml
oc apply -n bookinfo-1 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.3/samples/bookinfo/networking/bookinfo-gateway.yaml
# Telemetry API created in the control plane namespace is applied to all namespaces
oc apply -n istio-system-1 -f telemetry.yaml
```

TODO: Gateway injection does not work in this setup. Try with 2.3.1:
```shell
sed 's/{{host}}/httpbin-1/g' gateway-injection.yaml | oc apply -n httpbin-1 -f -
```

5. Request service in a loop to collect some metrics:
```shell
while true; do curl -v httpbin-1.apps-crc.testing:80/ > /dev/null; sleep 5; done
```

6. Configure monitoring:
```shell
oc apply -n istio-system-1 -f openshift-monitoring/istiod-monitor.yaml
oc apply -n istio-system-1 -f openshift-monitoring/istio-proxies-monitor.yaml
oc apply -n bookinfo-1 -f openshift-monitoring/istio-proxies-monitor.yaml
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
```

3. Login as `meshadmin-2` and deploy `Prometheus`:
```shell
oc login -u meshadmin-2 https://api.crc.testing:6443
oc apply -f custom-prometheus/prometheus.yaml
```

4. Deploy SMCP:
```shell
oc apply -n istio-system-2 -f custom-prometheus/mesh.yaml
sed 's/{{host}}/bookinfo-2/g' route.yaml | oc apply -n istio-system-2 -f -
```

Wait until istiod is ready and then apply:
```shell
oc apply -n bookinfo-2 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.3/samples/bookinfo/platform/kube/bookinfo.yaml
oc apply -n bookinfo-2 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.3/samples/bookinfo/networking/bookinfo-gateway.yaml
# Telemetry API created in the control plane namespace is applied to all namespaces
oc apply -n istio-system-2 -f telemetry.yaml
```

5. Enable monitoring:
```shell
sed "s/{{username}}/meshadmin-2/g" allow-admin-to-manage-telemetry-and-monitors.yaml | oc apply -n custom-prometheus -f -
oc apply -f custom-prometheus/istiod-monitor.yaml
oc apply -f custom-prometheus/istio-proxies-monitor.yaml
```

### Issues

1. SMCP must provide a way to enable telemetry.v2.prometheus.enabled without deploying Prometheus.
2. When mTLS is enabled in the mesh, Prometheus cannot scrape metrics from port 15090 with enabled TLS,
  because "server gave HTTP response to HTTPS client", as you can see in this [image](img/http-envoy-prom-tls.png).
  This is probably, because of excluding 15090 from inbound ports, because when TLS is not enabled in the PodMonitor,
  everything works fine, as you can see [here](img/http-envoy-prom.png). On the other hand, it does not work
  with OpenShift Monitoring when mTLS is enabled in the mesh and the PodMonitor does not use TLS.
