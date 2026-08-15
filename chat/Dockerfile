# syntax=docker/dockerfile:1

FROM bufbuild/buf:1.47.2 AS proto
WORKDIR /src
COPY buf.yaml buf.gen.yaml ./
COPY api ./api
RUN buf generate

FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates curl \
  && mkdir -p /out
ARG GRPC_HEALTH_PROBE_VERSION=v0.4.34
RUN curl -fsSL -o /out/grpc_health_probe \
  https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 \
  && chmod +x /out/grpc_health_probe
COPY go.mod go.sum ./
COPY --from=proto /src/gen ./gen
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/server /server
COPY --from=build /out/grpc_health_probe /grpc_health_probe
USER nonroot:nonroot
EXPOSE 50051 8081
ENTRYPOINT ["/server"]
