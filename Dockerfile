FROM golang:alpine AS builder

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download
ADD . .
RUN CGO_ENABLED=0 GOOS=linux go build ./cmd/app

FROM alpine:latest

RUN apk update && apk add --no-cache docker-cli-buildx git docker

COPY --from=builder /build/app /app

LABEL type="platform"

ENTRYPOINT ["/app"]
