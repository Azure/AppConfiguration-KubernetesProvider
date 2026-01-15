# Build the manager binary
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.24-fips-azurelinux3.0 AS builder
 
ARG MODULE_VERSION
WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
 
# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/controller/ internal/controller/
COPY internal/jsonc/ internal/jsonc/
COPY internal/loader/ internal/loader/
COPY internal/properties/ internal/properties/
 
ARG TARGETARCH
 
# Build
RUN GOOS=linux GOARCH=$TARGETARCH GOEXPERIMENT=systemcrypto go build -ldflags "-X azappconfig/provider/internal/properties.ModuleVersion=$MODULE_VERSION" -a -o manager cmd/main.go
 
# Use distroless as minimal base image to package the manager binary
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/azurelinux/distroless/minimal:3.0
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532
 
ENTRYPOINT ["/manager"]