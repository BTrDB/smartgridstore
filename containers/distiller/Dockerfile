FROM ubuntu:xenial

RUN apt-get update && apt-get install -y net-tools git build-essential vim wget librados-dev
RUN wget -O /tmp/go.tar.gz https://storage.googleapis.com/golang/go1.6.2.linux-amd64.tar.gz && tar -xf /tmp/go.tar.gz -C /usr/local/ && rm /tmp/go.tar.gz && mkdir /srv/go
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin:/srv/go/bin GOPATH=/srv/go
ENV GOTRACEBACK=all
ENV GOGC=40
RUN go get github.com/go-distil/distil
RUN go get gopkg.in/distil.v1
RUN go get github.com/maruel/panicparse/cmd/pp
RUN mkdir /wb
ADD entrypoint.sh /wb
WORKDIR /wb
ENTRYPOINT /wb/entrypoint.sh
