# Build the manager binary
FROM golang:1.23-alpine AS builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
ENV CGO_ENABLED=0
RUN go build -a -o conduit-operator cmd/operator/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /workspace/conduit-operator /app
USER 65532:65532

ENTRYPOINT ["/app/conduit-operator"]
