FROM alpine as builder

RUN apk update && apk add binutils ca-certificates && rm -rf /var/cache/apk/*

COPY ./build /build
COPY detect.sh /

RUN /detect.sh rest2dhcp

FROM alpine  

COPY --from=builder /rest2dhcp /bin/rest2dhcp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

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

# Run as UID for nobody since k8s pod securityContext runAsNonRoot can't resolve the user ID:
# https://github.com/kubernetes/kubernetes/issues/40958
USER 65534

ENTRYPOINT ["/bin/rest2dhcp"]
