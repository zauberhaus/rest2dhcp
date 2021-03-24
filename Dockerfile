FROM golang:1.16-alpine AS builder

RUN apk update && apk --no-cache upgrade && apk add --no-cache gcc musl-dev dep git ca-certificates tzdata busybox-static
RUN go install github.com/mjibson/esc@v0.2.0
RUN go install github.com/mdomke/git-semver@latest
RUN go get github.com/golang/mock/mockgen

COPY scripts/install_upx.sh /usr/local/bin 
RUN install_upx.sh

ENV SRC /go/src/github.com/zauberhaus/rest2dhcp
ENV TZ=Pacific/Auckland

RUN ln -s $SRC /src && mkdir -p /out
COPY . /src/
WORKDIR $SRC

RUN go generate ./service/server.go
RUN go build -ldflags "-X main.gitCommit=`git rev-parse --short HEAD` -X main.buildTime=`date -u -I'seconds'` -X main.treeState=`git diff --stat | grep "" > /dev/null  && echo dirty || echo clean` -X main.tag=`git semver -prefix v` -linkmode external -extldflags -static -s -w" -o /out/rest2dhcp
RUN upx --best /out/rest2dhcp

FROM scratch
COPY --from=0 /out/rest2dhcp /rest2dhcp

ENV CONFIG=
ENV MODE=
ENV CLIENT=
ENV SERVER=
ENV RELAY=
ENV QUIET=
ENV VERBOSE=
ENV LISTEN=
ENV KUBECONFIG=
ENV NAMESPACE=
ENV SERVICE=
ENV TIMEOUT=
ENV DHCP_SERVER=
ENV DHCP_TIMEOUT=
ENV RETRY=
ENV ACCESS_LOG=

EXPOSE 8080 67/udp 68/udp

CMD ["/rest2dhcp"]
