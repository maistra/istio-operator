cd ../../.. && IMAGE=${USER}/istio-operator ./tmp/build/docker_build.sh
docker images | grep ${USER}/istio-operator
