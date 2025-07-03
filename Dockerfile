FROM golang:alpine AS builder

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download
ADD . .
RUN CGO_ENABLED=0 GOOS=linux go build ./cmd/app

# Trivy workarround this should go back to alpine:latest
# when
# docker buildx and containerd image store are default in docker engine
FROM aquasec/trivy:latest

RUN apk update && apk add --no-cache docker-cli-buildx git docker openrc

COPY --from=builder /build/app /app

LABEL type="platform"

ENTRYPOINT ["/app"]
