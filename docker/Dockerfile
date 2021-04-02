FROM golang:1.16-alpine AS builder

RUN apk update && apk --no-cache upgrade && apk add --no-cache gcc musl-dev dep git ca-certificates tzdata busybox-static
RUN go install github.com/mjibson/esc@v0.2.0
RUN go install github.com/mdomke/git-semver@latest

ENV SRC /go/src/github.com/zauberhaus/rest2dhcp
ENV TZ=Pacific/Auckland

RUN ln -s $SRC /src
WORKDIR $SRC

CMD ["go", "run", "github.com/onsi/ginkgo/ginkgo"]