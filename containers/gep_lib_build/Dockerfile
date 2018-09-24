FROM ubuntu:bionic

RUN apt-get update && apt-get install -y build-essential cmake wget git zlib1g-dev
ADD entrypoint.sh /bin
ENTRYPOINT ["/bin/entrypoint.sh"]
