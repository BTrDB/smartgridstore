
FROM ubuntu:xenial

RUN apt-get update && apt-get install -y net-tools git build-essential vim wget
RUN wget -O /tmp/go.tar.gz https://storage.googleapis.com/golang/go1.6.2.linux-amd64.tar.gz && tar -xf /tmp/go.tar.gz -C /usr/local/ && rm /tmp/go.tar.gz && mkdir /srv/go
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin:/srv/go/bin GOPATH=/srv/go
ENV GOTRACEBACK=all
ENV GOGC=40
RUN apt-get install -y python-pip librados-dev libssl-dev libffi-dev python-dev
RUN go get github.com/SoftwareDefinedBuildings/mr-plotter
RUN go get github.com/maruel/panicparse/cmd/pp
RUN pip install pymongo requests bcrypt
RUN apt-get install -y letsencrypt
ADD entrypoint.sh /
RUN chmod a+x /entrypoint.sh

VOLUME /etc/mrplotter
ENTRYPOINT ["/entrypoint.sh"]
