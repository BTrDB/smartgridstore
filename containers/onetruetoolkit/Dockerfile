FROM ubuntu:xenial

ENV ETCDCTL_VERSION v3.1.5
ENV ETCDCTL_ARCH linux-amd64

RUN apt-get update && apt-get install -y wget && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    wget -q -O- "https://github.com/coreos/etcd/releases/download/${ETCDCTL_VERSION}/etcd-${ETCDCTL_VERSION}-${ETCDCTL_ARCH}.tar.gz" |tar xfz - -C/tmp/ && \
    mv /tmp/etcd-${ETCDCTL_VERSION}-${ETCDCTL_ARCH}/etcd* /bin/

RUN apt-get update && apt-get install -y net-tools git build-essential vim wget librados-dev
    RUN wget -O /tmp/go.tar.gz https://storage.googleapis.com/golang/go1.8.3.linux-amd64.tar.gz && tar -xf /tmp/go.tar.gz -C /usr/local/ && rm /tmp/go.tar.gz && mkdir /srv/go
    ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin:/srv/go/bin GOPATH=/srv/go
    ENV GOTRACEBACK=all
    RUN go get github.com/maruel/panicparse/cmd/pp
    WORKDIR /srv/go
ADD entrypoint.sh /bin/
RUN chmod a+x /bin/entrypoint.sh
RUN apt-get install -y byobu iputils-ping dns-utils curl silversearcher-ag
ENTRYPOINT ["/bin/entrypoint.sh"]
