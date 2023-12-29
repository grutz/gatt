# syntax=docker/dockerfile:1

ARG GO_VERSION=1.21
FROM golang:${GO_VERSION} as build

WORKDIR $GOPATH/src/gatt

# Environment
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR $GOPATH/src

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    go build -v ./...

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    go test -v ./...