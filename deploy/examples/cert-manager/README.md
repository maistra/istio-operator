# Integration with cert-manager-istio-csr

1. Install cert-manager
```shell
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.crds.yaml
helm install \
    cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --version v1.11.0
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
    openssl s_client -showcerts istiod-test-prototype.istio-system:15012 < /dev/null
```
2. Check app certificate:
```shell
kubectl exec $(kubectl get pods -l app=productpage -o jsonpath='{.items[0].metadata.name}') -c istio-proxy -- \
    openssl s_client -showcerts details.bookinfo:9080 < /dev/null
```
3. Check certificates with `istioctl`:
```shell
istioctl pc s $(kubectl get pods -l app=productpage -o jsonpath='{.items[0].metadata.name}') -o json |\
    jq -r '.dynamicActiveSecrets[0].secret.tlsCertificate.certificateChain.inlineBytes' |\
    base64 --decode | openssl x509 -text -noout
```
4. Verify by seeing all mounts are correct on pilot, launching bookinfo and checking.
5. It's also important to verify that the right root configmap is used for the root of trust.
