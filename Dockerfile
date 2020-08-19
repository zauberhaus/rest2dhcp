FROM golang:1.14-alpine AS builder

RUN apk update && apk --no-cache upgrade && apk add --no-cache gcc musl-dev dep git ca-certificates tzdata busybox-static

ENV SRC /go/src/github.com/zauberhaus/rest2dhcp
ENV TZ=Pacific/Auckland

RUN ln -s $SRC /src && mkdir -p /out
COPY . /src/
WORKDIR $SRC

RUN go get github.com/mjibson/esc
RUN go generate ./...
RUN go build -ldflags "-X main.gitCommit=`git rev-parse --short HEAD` -X main.buildTime=`date -u -I'seconds'` -X main.treeState=`git diff --stat | grep "" > /dev/null  && echo dirty || echo clean` -X main.tag=`git describe --tags 2> /dev/null` -linkmode external -extldflags -static -s -w" -o /out/rest2dhcp
RUN ./scripts/install_upx.sh && upx /out/rest2dhcp

FROM scratch
COPY --from=0 /out/rest2dhcp /rest2dhcp

ENV SERVER ""
ENV CLIENT ""
ENV RELAY ""
ENV MODE ""

EXPOSE 8080 67/udp

CMD ["/rest2dhcp"]
