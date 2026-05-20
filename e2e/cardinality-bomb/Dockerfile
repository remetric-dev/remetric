# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26.3-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath -ldflags "-s -w" -o /out/bomb .

FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.title="cardinality-bomb"
LABEL org.opencontainers.image.source="https://github.com/remetric-dev/remetric"
LABEL org.opencontainers.image.licenses="Apache-2.0"
COPY --from=build /out/bomb /bomb
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/bomb"]
