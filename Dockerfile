ARG UBI_MINIMAL
ARG UBI_MICRO

FROM --platform=$TARGETPLATFORM docker.jfrog.io/ubi9/ubi-minimal:${UBI_MINIMAL} AS ubi-micro-build
RUN mkdir -p /mnt/rootfs /etc/dnf /var/cache/microdnf; touch /etc/dnf/dnf.conf;
RUN microdnf --noplugins --config=/etc/dnf/dnf.conf --setopt=cachedir=/var/cache/microdnf --setopt=reposdir=/etc/yum.repos.d --setopt=varsdir=/etc/dnf --installroot=/mnt/rootfs --releasever=35 install net-tools shadow-utils curl tar zip hostname procps gawk libstdc++ libstdc++-devel --setopt install_weak_deps=1 --nodocs -y; microdnf clean all;
RUN microdnf --noplugins --config=/etc/dnf/dnf.conf --setopt=cachedir=/var/cache/microdnf --setopt=reposdir=/etc/yum.repos.d --setopt=varsdir=/etc/dnf --installroot=/mnt/rootfs --releasever=35 clean all --setopt install_weak_deps=1;
RUN rm -rf /mnt/rootfs/var/cache/*
RUN rm -fr /mnt/rootfs/usr/share/gcc-11/python \
    /mnt/rootfs/usr/share/gdb/auto-load/usr/lib64/__pycache__/libstdc++.so.6.0.25-gdb.cpython*

FROM --platform=$TARGETPLATFORM docker.jfrog.io/ubi9/ubi-micro:${UBI_MICRO} AS ubi9-micro
COPY --from=ubi-micro-build /mnt/rootfs/ /

ARG TARGETARCH

# Copy the go source
COPY bin/operator-${TARGETARCH} /operator

RUN chmod +x /operator

ENTRYPOINT ["/operator"]
