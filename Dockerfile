# syntax = docker.io/docker/dockerfile:1.27

ARG ALPINE_BASE=docker.io/library/alpine:3.24
ARG GOLANG_BASE=docker.io/library/golang:1.27-alpine3.24


FROM --platform=$BUILDPLATFORM $GOLANG_BASE AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=auto
ARG TARGETPLATFORM
RUN \
  --mount=type=bind,target=/mnt/randomizer \
  --mount=type=cache,id=randomizer.go-pkg,target=/go/pkg \
  --mount=type=cache,id=randomizer.go-build,target=/root/.cache/go-build \
  cd /mnt/randomizer && \
  source ./targetplatform-go-env.sh && \
  go build -v \
    -mod=vendor \
    -ldflags='-s -w' \
    -o /randomizer-server \
    ./cmd/randomizer-server


FROM $ALPINE_BASE AS server-image
COPY --link --from=build /randomizer-server /usr/local/bin/randomizer-server
EXPOSE 7636
ENTRYPOINT ["/usr/local/bin/randomizer-server"]
