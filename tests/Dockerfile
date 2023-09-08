FROM golang:1.21.0-alpine AS builder

RUN apk update && apk --no-cache upgrade && apk add --no-cache gcc musl-dev dep git ca-certificates tzdata busybox-static
RUN go install github.com/onsi/ginkgo/ginkgo@v1.15.2

ENV SRC /go/src/github.com/zauberhaus/rest2dhcp
ENV TZ=Pacific/Auckland

RUN ln -s $SRC /src
WORKDIR $SRC

COPY ./entrypoint.sh /entrypoint.sh

ENTRYPOINT [ "/entrypoint.sh" ]