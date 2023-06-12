# Build the manager binary
FROM golang:1.20 as builder
ARG TARGETOS
ARG TARGETARCH
ARG GIT_REVISION
ARG GIT_TAG
ARG GIT_STATUS

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY api/ api/
COPY pkg/ pkg/
COPY controllers/ controllers/
COPY main.go main.go
COPY Makefile Makefile

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN make build -e GOOS=${TARGETOS:-linux} -e GOARCH=${TARGETARCH}

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest 
# gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/bin/manager .
COPY resources /var/lib/istio-operator/resources
USER 65532:65532

ENTRYPOINT ["/manager"]
