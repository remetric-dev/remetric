# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26.3-alpine AS build
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/remetric ./cmd/remetric

FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.title="remetric"
LABEL org.opencontainers.image.source="https://github.com/remetric-dev/remetric"
LABEL org.opencontainers.image.licenses="Apache-2.0"
COPY --from=build /out/remetric /remetric
USER 65532:65532
ENTRYPOINT ["/remetric"]
