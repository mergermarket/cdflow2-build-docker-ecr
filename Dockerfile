ARG WINBASE

FROM --platform=$BUILDPLATFORM golang:alpine AS build
ARG TARGETARCH
ARG TARGETOS
ARG TARGETVARIANT
WORKDIR /src

RUN apk add -U ca-certificates git

RUN --mount=target=./go.mod,source=./go.mod \
    --mount=target=./go.sum,source=./go.sum \
     go mod download

RUN --mount=target=. \
    if [ "$TARGETARCH" = "arm" ]; then export GOARM="${TARGETVARIANT//v}"; fi; \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app ./cmd/app


FROM alpine:latest AS linux

RUN apk update && apk add --no-cache docker-cli-buildx

COPY --from=build /app /app
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
VOLUME /tmp
ENV TMPDIR /tmp

LABEL type="platform"
ENTRYPOINT ["/app"]


FROM ${WINBASE} AS windows
COPY --from=build /app app.exe

LABEL type="platform"
ENTRYPOINT [ "app.exe" ]
