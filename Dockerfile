ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest
LABEL maintainer="The Prometheus Authors <prometheus-developers@googlegroups.com>"

ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/gaussdb_exporter /bin/gaussdb_exporter

EXPOSE     9187
USER       nobody
ENTRYPOINT [ "/bin/gaussdb_exporter" ]
