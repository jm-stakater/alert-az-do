# TODO
FROM golang:1.19 AS builder
WORKDIR /go/src/github.com/jm-stakater/alert-az-do
COPY . /go/src/github.com/jm-stakater/alert-az-do
RUN GO111MODULE=on GOBIN=/tmp/bin make

FROM quay.io/prometheus/busybox-linux-amd64:latest

COPY --from=builder /go/src/github.com/jm-stakater/alert-az-do/alert-az-do /bin/alert-az-do

ENTRYPOINT [ "/bin/alert-az-do" ]
