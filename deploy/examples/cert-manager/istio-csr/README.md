## Integration with cert-manager and istio-csr

#### Prerequisites

1. Install cert-manager operator (community v1.11.x+ or provided by Red Hat v1.10.x).
2. Install OpenShift Service Mesh operator (v2.4+).

#### Steps

1. Provision certificates:
```shell
oc apply -f deploy/examples/cert-manager/istio-csr/selfsigned-ca.yaml
```

> **Note**
>
> Please note that the namespace of `selfsigned-root-issuer` issuer and `root-ca` certificate is `cert-manager-operator`.
> This is necessary, because `root-ca` is a cluster issuer, so cert-manager will look for a referenced secret 
> in its own namespace, which is `cert-manager-operator` in case of cert-manager provided by Red Hat.

2. Install cert-manager istio-csr:
```shell
oc new-project istio-system
helm install istio-csr jetstack/cert-manager-istio-csr \
    -n istio-system \
    -f deploy/examples/cert-manager/istio-csr/istio-csr.yaml
```

> **Note**
>
> We test integration with cert-manager v0.5.x+. If you want to integrate OSSM with
> a previous version of istio-csr, make sure that it will provide:
> 
> * a secret of type `kubernetes.io/tls` named `istiod-tls` that contains a root certificate
> and an intermediate CA key and certificate for istiod;
> * a config map named `istio-ca-root-cert` that contains the root certificate under key `root-cert.pem`.
>
> Both these resources must exist in the same namespace as an SMCP.

3. Deploy Istio:
```shell
oc apply -f deploy/examples/cert-manager/istio-csr/mesh.yaml -n istio-system
```

4. Deploy bookinfo app:
```shell
oc new-project bookinfo
oc apply -f https://raw.githubusercontent.com/maistra/istio/maistra-2.3/samples/bookinfo/platform/kube/bookinfo.yaml -n bookinfo
```

### Integration with cert-manager in OSSM 2.3.0 and 2.3.1

To make SMCP 2.3.0 and 2.3.1 work with cert-manager it's required to manually adjust certificate provided by cert-manager.

1. Follow steps 1-3 from the previous section.

2. Get intermediate certificate for Istio provided by cert-manager:
```shell
oc get secret istiod-tls -n istio-system -o json | jq -r '.data."tls.crt"' | base64 -d > ca-cert.pem
```

3. Create secret for Istio Operator from retrieved certificate:
```shell
oc create secret generic istio-ca-secret -n istio-system --from-file=ca-cert.pem
```

4. Deploy Istio:
```shell
oc apply -f deploy/examples/cert-manager/istio-csr/smcp-2.3.1.yaml -n istio-system
```

5. Test traffic:
```shell
oc apply -f route.yaml -n deploy/examples/cert-manager/istio-csr/istio-system
while true; do curl -v bookinfo.apps-crc.testing:80/productpage > /dev/null; sleep 1; done
```

### Verification

1. Check istiod certificate:
```shell
kubectl exec $(kubectl get pods -l app=productpage -o jsonpath='{.items[0].metadata.name}') -c istio-proxy -- \
    openssl s_client -showcerts istiod-basic.istio-system:15012 < /dev/null
```
2. Check app certificate:
```shell
kubectl exec $(kubectl get pods -l app=productpage -o jsonpath='{.items[0].metadata.name}') -c istio-proxy -- \
    openssl s_client -showcerts details.bookinfo:9080 < /dev/null
```
3. Verify by seeing all mounts are correct on pilot, launching bookinfo and checking.
4. It's also important to verify that the right root configmap is used for the root of trust.
