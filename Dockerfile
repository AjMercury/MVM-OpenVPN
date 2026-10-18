# Legacy Linux Docker entry point; prefer editions/linux/docker/Dockerfile.
ARG GO_VERSION=1.27.2
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT
ARG VERSION=0.2.0-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -trimpath -tags nomsgpack -ldflags="-s -w -X main.version=$VERSION" -o /out/server ./cmd/server

FROM alpine:3.22 AS certificates
RUN apk add --no-cache ca-certificates

FROM scratch
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/server /server
USER 65532:65532
WORKDIR /data
ENV MVM_LISTEN=0.0.0.0:8080 MVM_DATABASE=/data/mvm.db
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD ["/server", "healthcheck"]
ENTRYPOINT ["/server"]

