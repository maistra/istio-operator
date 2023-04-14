## Integration with cert-manager and istio-csr

#### Prerequisites

1. Install cert-manager operator (v1.11.x+).
2. Install OpenShift Service Mesh operator (v2.4+).

#### Steps

1. Create root cluster issuer:
```shell
oc apply -f cluster-issuer.yaml
```

> **Note**
> 
> Please note that the namespace of `selfsigned-root-issuer` issuer and `root-ca` certificate is `openshift-operators`.
> This is necessary, because `root-ca` is a cluster issuer, so cert-manager will look for a referenced secret in its own namespace,
> which is `openshift-operators` in this case.

2. Create an intermediate CA certificate for istiod:
```shell
oc new-project istio-system
oc apply -f cacerts.yaml
```

3. Deploy SMCP:
```shell
oc apply -f mesh.yaml -n istio-system
```

4. Deploy apps:
```shell
oc new-project sleep
oc apply -f https://raw.githubusercontent.com/maistra/istio/maistra-2.4/samples/sleep/sleep.yaml -n sleep
oc new-project httpbin
oc apply -f https://raw.githubusercontent.com/maistra/istio/maistra-2.4/samples/httpbin/httpbin.yaml -n httpbin
```

5. Test mTLS between apps:
```shell
kubectl exec $(kubectl get pods -n sleep -l app=sleep -o jsonpath='{.items[0].metadata.name}') -n sleep -c sleep -- \
    curl -v http://httpbin.httpbin.svc:8000/headers
```

6. Test connection and check certs:
```shell
kubectl exec $(kubectl get pods -n httpbin -l app=httpbin -o jsonpath='{.items[0].metadata.name}') -n httpbin -c istio-proxy -- \
    openssl s_client -showcerts istiod-basic.istio-system:15012 < /dev/null > istiod-cert.out
kubectl exec $(kubectl get pods -n httpbin -l app=httpbin -o jsonpath='{.items[0].metadata.name}') -n httpbin -c istio-proxy -- \
    openssl s_client -showcerts istiod-basic.istio-system:443 < /dev/null > istiod-webhook-cert.out
```

7. Trigger CA rotation:
```shell
kubectl patch certificates cacerts -n istio-system --type=merge -p '{"spec":{"duration":"720h"}}'
```

8. Istio operator should receive event with secret update and reconcile webhooks:
```log
{"level":"info","ts":1680788982.2718391,"logger":"webhookca-controller","msg":"reconciling WebhookConfiguration","WebhookConfig":"mutating/istiod-basic-istio-system"}
{"level":"info","ts":1680788982.2719076,"logger":"webhookca-controller","msg":"Updating CABundle","WebhookConfig":"mutating/istiod-basic-istio-system"}
{"level":"info","ts":1680788982.2763023,"logger":"webhookca-controller","msg":"CABundle updated","WebhookConfig":"mutating/istiod-basic-istio-system"}
{"level":"info","ts":1680788982.27635,"logger":"webhookca-controller","msg":"reconciling WebhookConfiguration","WebhookConfig":"validating/istio-validator-basic-istio-system"}
{"level":"info","ts":1680788982.2764895,"logger":"webhookca-controller","msg":"Updating CABundle","WebhookConfig":"validating/istio-validator-basic-istio-system"}
{"level":"info","ts":1680788982.2896943,"logger":"webhookca-controller","msg":"CABundle updated","WebhookConfig":"validating/istio-validator-basic-istio-system"}
{"level":"info","ts":1680788982.2897654,"logger":"webhookca-controller","msg":"reconciling WebhookConfiguration","WebhookConfig":"mutating/istiod-basic-istio-system"}
{"level":"info","ts":1680788982.2899144,"logger":"webhookca-controller","msg":"Correct CABundle already present. Ignoring","WebhookConfig":"mutating/istiod-basic-istio-system"}
{"level":"info","ts":1680788982.2899542,"logger":"webhookca-controller","msg":"reconciling WebhookConfiguration","WebhookConfig":"validating/istio-validator-basic-istio-system"}
{"level":"info","ts":1680788982.2900114,"logger":"webhookca-controller","msg":"Correct CABundle already present. Ignoring","WebhookConfig":"validating/istio-validator-basic-istio-system"}
```

9. Istiod should notice secret update and reload its certificate:
```log
2023-04-06T13:50:22.864015Z info Update Istiod cacerts
2023-04-06T13:50:22.864105Z info Using kubernetes.io/tls secret type for signing ca files
2023-04-06T13:50:22.995853Z info Istiod has detected the newly added intermediate CA and updated its key and certs accordingly
2023-04-06T13:50:22.996343Z info x509 cert - Issuer: "CN=istiod-basic.istio-system.svc", Subject: "", SN: b1f27d58ac923f479d3fd4cefa1c6cc2, NotBefore: "2023-04-06T13:48:22Z", NotAfter: "2033-04-03T13:50:22Z"
2023-04-06T13:50:22.996433Z info x509 cert - Issuer: "CN=root-ca.my-company.net,O=my-company.net", Subject: "CN=istiod-basic.istio-system.svc", SN: 1058e1403d70948ea54a4c6bd5d05dbd, NotBefore: "2023-04-06T13:49:42Z", NotAfter: "2023-05-14T01:49:42Z"
2023-04-06T13:50:22.996468Z info x509 cert - Issuer: "CN=root-ca.my-company.net,O=my-company.net", Subject: "CN=root-ca.my-company.net,O=my-company.net", SN: 352d9e6c05c500d887b433e367f97273, NotBefore: "2023-04-05T14:35:01Z", NotAfter: "2025-09-21T14:35:01Z"
2023-04-06T13:50:22.996476Z info Istiod certificates are reloaded
```
