# Monitoring solutions for OpenShift Service Mesh

### Prepare users and permissions

1. Prepare htpasswd identity provider
```shell
touch htpasswd
htpasswd -Bb htpasswd clusteradmin clusteradm
htpasswd -Bb htpasswd meshadmin-1 adm1
htpasswd -Bb htpasswd meshadmin-2 adm2
htpasswd -Bb htpasswd meshadmin-3 adm3
htpasswd -Bb htpasswd meshadmin-4 adm4
htpasswd -Bb htpasswd meshadmin-5 adm5
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

for i in 1 2 3 4 5
do
  oc new-project istio-system-$i
  oc new-project httpbin-$i
  oc create user meshadmin-$i
  oc create identity simple-htpasswd:meshadmin-$i
  oc create useridentitymapping simple-htpasswd:meshadmin-$i meshadmin-$i
  oc adm policy add-role-to-user admin meshadmin-$i -n istio-system-$i
  oc adm policy add-role-to-user admin meshadmin-$i -n httpbin-$i
done
```

3. Try to login as `clusteradmin`:
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

2. Grant users permission to use Prometheus monitors:
```shell
oc apply -n istio-system-1 -f openshift-monitoring/role.yaml
oc apply -n httpbin-1 -f openshift-monitoring/role.yaml
sed 's/{{targetUser}}/meshadmin-1/g' openshift-monitoring/role-binding.yaml | oc apply -n istio-system-1 -f -
sed 's/{{targetUser}}/meshadmin-1/g' openshift-monitoring/role-binding.yaml | oc apply -n httpbin-1 -f -
```

3. Install OpenShift Service Mesh operator.

4. Deploy control plane and an app for the first tenant:
```shell
oc login -u meshadmin-1 https://api.crc.testing:6443
sed 's/{{memberNamespace}}/httpbin-1/' openshift-monitoring/mesh.yaml | oc apply -n istio-system-1 -f -
oc apply -n httpbin-1 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.3/samples/httpbin/httpbin.yaml
sed 's/{{host}}/httpbin-1/g' gateway.yaml | oc apply -n httpbin-1 -f -
sed 's/{{host}}/httpbin-1/g' route.yaml | oc apply -n istio-system-1 -f -
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
oc apply -n httpbin-1 -f openshift-monitoring/istio-proxies-monitor.yaml
```

## SMCP addons

```shell
oc new-project mesh-1
oc new-project bookinfo-1

oc apply -n mesh-1 -f addons/mesh.yaml
oc apply -n bookinfo-1 -f https://raw.githubusercontent.com/maistra/istio/maistra-2.3/samples/bookinfo/platform/kube/bookinfo.yaml
```
