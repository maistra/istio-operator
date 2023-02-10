## Integration with cert-manager and istio-csr

#### Requirements

OpenShift Service Mesh requires cert-manager to provide:
* a TLS secret with an intermediate CA key and certificate for istiod:
    * name: `istiod-tls`;
    * namespace: the same one in which SMCP is deployed;
    * data keys: `tls.key` and `tls.crt`.
* a config map with a root certificate:
    * name: `istio-ca-root-cert` or custom specified in `spec.security.certificateAuthority.cert-manager.rootCAConfigMapName`;
    * namespace: the same one in which SMCP is deployed;
    * data key: `root-cert.pem`.

Currently, istio-csr does not allow customizing namespace in which secrets and config maps are created,
but if it will change in the future, OSSM users will have to configure istio-csr to provide objects named as described above.

#### Steps

1. Install cert-manager
```shell
helm install cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --version v1.11.0 \
    --set installCRDs=true
```

2. Provision certificates:
```shell
oc new-project istio-system
oc apply -f deploy/examples/cert-manager/selfsigned-ca.yaml -n istio-system
```

3. Install cert-manager istio-csr Service:
```shell
helm install -n cert-manager cert-manager-istio-csr \
    jetstack/cert-manager-istio-csr -f deploy/examples/cert-manager/istio-csr-helm-values.yaml
```

4. Deploy Istio:
```shell
oc apply -f deploy/examples/cert-manager/smcp.yaml -n istio-system
```

5. Deploy bookinfo app:
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
oc apply -f deploy/examples/cert-manager/smcp-2.3.1.yaml -n istio-system
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
