FROM ubuntu:xenial

ENV CEPH_VERSION jewel

RUN apt-get update && apt-get install -y wget && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
ENV GOTRACEBACK=all
ADD c37ingress /bin/
ENTRYPOINT ["/bin/c37ingress"]
