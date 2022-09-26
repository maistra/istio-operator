# Steps

## Install cert-manager
    helm install \
        cert-manager jetstack/cert-manager \
        --namespace cert-manager \
        --create-namespace \
        --version v1.6.1 \
        --set installCRDs=true

## Prepare istio-system Namespace

1. Create system namespace:

        kubectl create ns istio-system

2. Provision certificates:

        kubectl apply -f selfsigned-ca.yaml -n istio-system

3. Install cert-manager istio-csr Service:

        helm install -n cert-manager cert-manager-istio-csr jetstack/cert-manager-istio-csr -f deploy-prototype/istio-csr-helm-values.yaml"

## Create Istio Control Plane

    kubectl apply -f deploy/examples/cert-manager/smcp.yaml

## Verify

The smcp should proceed and istio-csr should be configured properly!

1. Verify by seeing all mounts are correct on pilot, launching bookinfo and checking
2. `istioctl pc s <pod-name>` to see our self-signed ca is the issuer.
3. It's also important to verify that the right root configmap is used for the root of trust.
