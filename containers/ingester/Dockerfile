FROM ubuntu:bionic

ENV CEPH_VERSION mimic

RUN apt-get update && apt-get install -y wget gnupg && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    echo "deb http://download.ceph.com/debian-${CEPH_VERSION}/ bionic main" | tee /etc/apt/sources.list.d/ceph-${CEPH_VERSION}.list && \
    apt-get update && apt-get install -y librados-dev && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
ENV GOTRACEBACK=all
ADD ingester /bin/
ENTRYPOINT ["/bin/ingester"]
