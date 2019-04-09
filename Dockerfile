FROM golang:1.11.1-alpine3.8 as builder
WORKDIR /go/src/github.com/sapcc/k8s-secrets-certificate-exporter
RUN apk add --no-cache make
COPY . .
ARG VERSION
RUN make all

FROM alpine:3.8
MAINTAINER Arno Uhlig <arno.uhlig@@sap.com>

RUN apk add --no-cache curl tini
RUN tini --version
COPY --from=builder /go/src/github.com/sapcc/k8s-secrets-certificate-exporter/bin/linux/exporter /usr/local/bin/
ENTRYPOINT ["tini", "--"]
CMD ["exporter"]
