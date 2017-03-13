FROM ubuntu:xenial

ENV ETCDCTL_VERSION v3.1.3
ENV ETCDCTL_ARCH linux-amd64

RUN apt-get update && apt-get install -y wget && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    wget -q -O- "https://github.com/coreos/etcd/releases/download/${ETCDCTL_VERSION}/etcd-${ETCDCTL_VERSION}-${ETCDCTL_ARCH}.tar.gz" |tar xfz - -C/tmp/ && \
    mv /tmp/etcd-${ETCDCTL_VERSION}-${ETCDCTL_ARCH}/etcd* /bin/
ADD entrypoint.sh /
ENTRYPOINT ["/entrypoint.sh"]
