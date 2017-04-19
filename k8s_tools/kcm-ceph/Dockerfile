FROM ubuntu:xenial

ENV ETCDCTL_VERSION v3.1.3
ENV ETCDCTL_ARCH linux-amd64
ENV CEPH_VERSION jewel
ENV KUBE_VERSION v1.6.1

RUN apt-get update && apt-get install -y net-tools git build-essential wget
RUN wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    echo "deb http://download.ceph.com/debian-${CEPH_VERSION}/ xenial main" | tee /etc/apt/sources.list.d/ceph-${CEPH_VERSION}.list && \
    apt-get update && apt-get install -y --force-yes ceph radosgw librados-dev rbd-mirror vim net-tools git build-essential && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    wget -q -O- "https://github.com/coreos/etcd/releases/download/${ETCDCTL_VERSION}/etcd-${ETCDCTL_VERSION}-${ETCDCTL_ARCH}.tar.gz" |tar xfz - -C/tmp/ etcd-${ETCDCTL_VERSION}-${ETCDCTL_ARCH}/etcdctl && \
    mv /tmp/etcd-${ETCDCTL_VERSION}-${ETCDCTL_ARCH}/etcdctl /usr/local/bin/etcdctl
RUN wget -O /tmp/go.tar.gz https://storage.googleapis.com/golang/go1.7.4.linux-amd64.tar.gz && tar -xf /tmp/go.tar.gz -C /usr/local/ && rm /tmp/go.tar.gz && mkdir /srv/go
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin:/srv/target:/srv/go/bin GOPATH=/srv/go
RUN go get k8s.io/kubernetes/cmd/kube-controller-manager && \
    cd $GOPATH/src/k8s.io/kubernetes/cmd/kube-controller-manager && \
    git checkout ${KUBE_VERSION} && \
    go install && \
    cd / && \
    rm -rf $GOPATH/src/k8s.io
