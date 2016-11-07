
FROM ceph/daemon:tag-build-master-jewel-ubuntu-16.04

RUN apt-get update && apt-get install -y net-tools git build-essential librados-dev vim iputils-ping
RUN wget -O /tmp/go.tar.gz https://storage.googleapis.com/golang/go1.6.3.linux-amd64.tar.gz && tar -xf /tmp/go.tar.gz -C /usr/local/ && rm /tmp/go.tar.gz && mkdir /srv/go
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin:/srv/target:/srv/go/bin:/wb/bin GOPATH=/srv/go ETCDCTL_API=2
ENV GOTRACEBACK=all
ENV GOGC=40
ADD btrdbd /bin/
RUN apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv EA312927 && \
    echo "deb http://repo.mongodb.org/apt/ubuntu trusty/mongodb-org/3.2 multiverse" > /etc/apt/sources.list.d/mongodb-org-3.2.list && \
    apt-get update && \
    apt-get install -y mongodb-org byobu
RUN apt-get install -y mysql-client
RUN go get github.com/SoftwareDefinedBuildings/mr-plotter
RUN wget https://github.com/coreos/etcd/releases/download/v3.0.4/etcd-v3.0.4-linux-amd64.tar.gz && tar -xf etcd-v3.0.4-linux-amd64.tar.gz && cp etcd-v3.0.4-linux-amd64/etc* /bin/ && rm -rf etcd*
RUN apt-get install -y python-pip librados-dev libssl-dev libffi-dev python-dev
RUN pip install pymongo requests bcrypt configobj pytz
RUN apt-get install -y ipython letsencrypt
ADD entrypoint.sh /
RUN chmod a+x /entrypoint.sh
WORKDIR /wb
ADD accounts /wb
ADD upmu-adm /wb
ADD mongo.sh /wb
ADD set-selfsigned-certs.sh /wb
ADD manager2lite.py /wb

ENTRYPOINT ["/entrypoint.sh"]
