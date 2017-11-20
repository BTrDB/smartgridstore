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
  if docker rm -f ${CONTAINER_PREFIX}ceph-${suffix} >/dev/null 2>&1
  then
    echo " DELETED"
  else
    echo " DID NOT EXIST"
  fi
done

echo "[INFO] removing container ${CONTAINER_PREFIX}etcd if exists"
if docker rm -f ${CONTAINER_PREFIX}etcd >/dev/null 2>&1
then
  echo " DELETED"
else
  echo " DID NOT EXIST"
fi

echo "[INFO] removing container ${CONTAINER_PREFIX}btrdbd if exists"
if docker rm -f ${CONTAINER_PREFIX}btrdbd >/dev/null 2>&1
then
  echo " DELETED"
else
  echo " DID NOT EXIST"
fi

echo "[INFO] removing container ${CONTAINER_PREFIX}console if exists"
if docker rm -f ${CONTAINER_PREFIX}console >/dev/null 2>&1
then
  echo " DELETED"
else
  echo " DID NOT EXIST"
fi

echo "[INFO] removing container ${CONTAINER_PREFIX}apifrontend if exists"
if docker rm -f ${CONTAINER_PREFIX}apifrontend >/dev/null 2>&1
then
  echo " DELETED"
else
  echo " DID NOT EXIST"
fi

echo "[INFO] removing container ${CONTAINER_PREFIX}mrplotter if exists"
if docker rm -f ${CONTAINER_PREFIX}mrplotter >/dev/null 2>&1
then
  echo " DELETED"
else
  echo " DID NOT EXIST"
fi

echo "[INFO] removing network ${DOCKERNET} if exists"
if docker network rm ${DOCKERNET} >/dev/null 2>&1
then
  echo " DELETED"
else
  echo " DID NOT EXIST"
fi

echo "[INFO] removing all state"
# bad luck if OSDBASE is somehow empty despite the check above
rm -rf ${OSDBASE}/etc/*
rm -rf ${OSDBASE}/var/*
rm -rf ${OSDBASE}/osd*
rm -rf ${ETCDBASE}/*

echo "[OKAY] all done here, have a great day!"
