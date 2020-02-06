FROM golang:alpine AS builder

WORKDIR /build

ADD main.go go.mod go.sum ./
RUN unset GOPATH && go build .

FROM alpine:latest

RUN apk add --no-cache docker-cli

COPY --from=builder /build/cdflow-release-docker-ecr /cdflow-release-docker-ecr

ENTRYPOINT ["/cdflow-release-docker-ecr"]
