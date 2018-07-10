FROM ubuntu:xenial

ENV CEPH_VERSION jewel

RUN apt-get update && apt-get install -y wget && \
    wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - && \
    echo "deb http://download.ceph.com/debian-${CEPH_VERSION}/ xenial main" | tee /etc/apt/sources.list.d/ceph-${CEPH_VERSION}.list && \
    apt-get update && apt-get install -y librados-dev ceph && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
ENV GOTRACEBACK=all
ADD admincliserver /bin/
ENTRYPOINT ["/bin/admincliserver"]
