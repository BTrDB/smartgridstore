#!/bin/bash


function check_var {
  if [[ "${!1}" == "" ]]
  then
    echo "Please set \$$1"
    exit 1
  fi
}

# check the appropriate variables are set
check_var VERSION
check_var OSDBASE
check_var DOCKERNET
check_var CONTAINER_PREFIX
check_var SUB24
check_var HOTPOOL
check_var COLDPOOL

set -e
mkdir -p ${OSDBASE}
set +e

docker network inspect ${DOCKERNET} 2>&1 >/dev/null

if [[ $? != 0 ]]
then
  # maybe it doesn't exit
  # try create the network
  docker network create --subnet ${SUB24}.0/24 ${DOCKERNET}
  if [[ $? != 0 ]]
  then
    echo "[ABORT] could not create docker subnet"
    exit 1
  else
    echo "[OKAY ] docker network created"
  fi
else
  echo "[OKAY ] docker network exists"
fi

# if the ceph containers already exist, delete them
docker inspect ${CONTAINER_PREFIX}ceph-mon 2>&1 >/dev/null
if [[ $? == 0 ]]
then
  # the container exists
  docker rm -f ${CONTAINER_PREFIX}ceph-mon
  if [[ $? != 0 ]]
  then
    echo "monitor container exists, but could not kill it"
    exit 1
  fi
fi

docker inspect ${CONTAINER_PREFIX}ceph-mgr 2>&1 >/dev/null
if [[ $? == 0 ]]
then
  # the container exists
  docker rm -f ${CONTAINER_PREFIX}ceph-mgr
  if [[ $? != 0 ]]
  then
    echo "mgr container exists, but could not kill it"
    exit 1
  fi
fi

# if the ceph containers already exist, delete them
docker inspect ${CONTAINER_PREFIX}etcd 2>&1 >/dev/null
if [[ $? == 0 ]]
then
  # the container exists
  docker rm -f ${CONTAINER_PREFIX}etcd
  if [[ $? != 0 ]]
  then
    echo "etcd container exists, but could not kill it"
    exit 1
  fi
fi

for osdnum in 0 1 2 3
do
  docker inspect ${CONTAINER_PREFIX}ceph-osd-${osdnum} 2>&1 >/dev/null
  if [[ $? == 0 ]]
  then
    # the container exists
    docker rm -f ${CONTAINER_PREFIX}ceph-osd-${osdnum}
    if [[ $? != 0 ]]
    then
      echo "osd-${osdnum} container exists, but could not kill it"
      exit 1
    fi
  fi
done

set -x

# all containers are gone, lets create new ones
docker run -d --net ${DOCKERNET} --ip ${SUB24}.5 \
 --name ${CONTAINER_PREFIX}ceph-mon \
 -v ${OSDBASE}/etc/ceph:/etc/ceph \
 -v ${OSDBASE}/var/lib/ceph/:/var/lib/ceph/ \
 -e MON_IP=${SUB24}.5 \
 -e CEPH_PUBLIC_NETWORK=${SUB24}.0/24 \
 ceph/daemon mon

if [[ $? != 0 ]]
then
  echo "[ABORT] Could not start monitor container"
  exit 1
fi

echo "[OKAY ] waiting for monitor container to start"

sleep 1

while true
do
  if [[ ! -e ${OSDBASE}/etc/ceph/ceph.conf ]]
  then
    echo "[WARN] ceph config doesn't exist (monitor slow to start?)"
    sleep 1
  else
    echo "[OKAY] ceph config found"
    break
  fi
done

# if these parameters do not exist in the ceph config, we must add them
if ! grep -e "name len = 256" ${OSDBASE}/etc/ceph/ceph.conf
then
  echo "[WARN] inserting ext4 filename workaround and restarting mon"
  echo "osd max object name len = 256" >> ${OSDBASE}/etc/ceph/ceph.conf
  echo "osd max object namespace len = 64" >> ${OSDBASE}/etc/ceph/ceph.conf
  docker restart ${CONTAINER_PREFIX}ceph-mon
else
  echo "[OKAY] ext4 workaround found"
fi

docker run -d --net ${DOCKERNET} --ip ${SUB24}.4 \
 --name ${CONTAINER_PREFIX}ceph-mgr \
 -v ${OSDBASE}/etc/ceph:/etc/ceph \
 -v ${OSDBASE}/var/lib/ceph/:/var/lib/ceph/ \
 -e MON_IP=${SUB24}.5 \
 -e CEPH_PUBLIC_NETWORK=${SUB24}.0/24 \
 ceph/daemon mgr

if [[ $? != 0 ]]
then
  echo "[ABORT] Could not start mgr container"
  exit 1
fi

#
for osdnum in 0 1 2 3
do
  lastoctet=$(( 10 + $osdnum ))
  docker run -d --net ${DOCKERNET} --ip ${SUB24}.${lastoctet} \
    --name ${CONTAINER_PREFIX}ceph-osd-${osdnum} \
   -v ${OSDBASE}/etc/ceph:/etc/ceph \
   -v ${OSDBASE}/var/lib/ceph/:/var/lib/ceph/ \
   -v ${OSDBASE}/osd${osdnum}:/var/lib/ceph/osd \
   -e OSD_TYPE=directory \
   ceph/daemon osd
   sleep 2
done

# start etcd
docker run -d --net ${DOCKERNET} --ip ${SUB24}.20 \
  --name ${CONTAINER_PREFIX}etcd \
  -v ${ETCDBASE}/db:/var/lib/etcd \
  -e ETCD_DATA_DIR=/var/lib/etcd \
  -e ETCD_LISTEN_CLIENT_URLS=http://${SUB24}.20:2379 \
  -e ETCD_ADVERTISE_CLIENT_URLS=http://${SUB24}.20:2379 \
  btrdb/stubetcd:3.1.10

# check that the ceph pools are okay
if ! bin/dvceph osd pool get ${HOTPOOL} size
then
  # maybe it doesn't exist
  if ! bin/dvceph osd pool create ${HOTPOOL} 16 16
  then
    echo "could not create hot pool"
    exit 1
  fi
fi

if ! bin/dvceph osd pool get ${COLDPOOL} size
then
  # maybe it doesn't exist
  if ! bin/dvceph osd pool create ${COLDPOOL} 16 16
  then
    echo "could not create cold pool"
    exit 1
  fi
fi

# run the btrdb ensuredb command
# start the btrdb container
