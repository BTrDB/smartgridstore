FROM ubuntu:bionic

COPY thirdparty /thirdparty
COPY gepingress /
ENV LD_LIBRARY_PATH=/thirdparty/Libraries/boost

ENTRYPOINT ["/gepingress"]
