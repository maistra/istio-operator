# Maistra Istio Operator Integration Test

This integration test suite is similar to the upstream Istio integration tests. The scripts `lib.sh`, `kind_provisioner.sh` and `istio-integ-suite-kind.sh` are copied from `github.com/maistra/istio` repo.

## How to Run it Manually

1. Create a builder container:

```
$ docker pull quay.io/maistra-dev/maistra-builder:2.4
$ docker run -d -t --rm \
    --name test \
    --privileged \
    -v /var/lib/docker \
    quay.io/maistra-dev/maistra-builder:2.4 \
    entrypoint tail -f /dev/null
```

2. Copy this istio-operator source code into the container

```
$ cd $(git rev-parse --show-toplevel)
$ docker cp . test:/work
```

3. Run `operator-integ-suite-kind.sh` in the builder container

```
$ docker exec -it test /bin/bash
(root@) make test.integration.kind
```