#!/bin/bash

function check_var {
  if [[ "${!1}" == "" ]]
  then
    echo "Please set \$$1"
    exit 1
  fi
}

# check the appropriate variables are set
check_var CONTAINER_PREFIX
check_var OSDBASE
check_var DOCKERNET
check_var ETCDBASE

for suffix in osd-0 osd-1 osd-2 osd-3 mon mgr
do
  echo "[INFO] removing container ${CONTAINER_PREFIX}ceph-${suffix} if exists"
  docker rm -f ${CONTAINER_PREFIX}ceph-${suffix}
done

echo "[INFO] removing container  ${CONTAINER_PREFIX}etcd if exists"
docker rm -f ${CONTAINER_PREFIX}etcd

echo "[INFO] removing network ${DOCKERNET} if exists"
docker network rm ${DOCKERNET}


echo "[INFO] removing all state"
# bad luck if OSDBASE is somehow empty despite the check above
rm -rf ${OSDBASE}/etc/*
rm -rf ${OSDBASE}/var/*
rm -rf ${OSDBASE}/osd*
rm -rf ${ETCDBASE}/*

echo "[OKAY] done"
