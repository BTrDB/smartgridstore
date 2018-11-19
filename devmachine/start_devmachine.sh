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
check_var JOURNALPOOL
check_var PLOTTER_PORT
check_var CONSOLE_PORT
check_var API_GRPC_PORT
check_var API_HTTP_PORT

set -e
mkdir -p ${OSDBASE}
set +e
#
# FS=$(stat -f -c %T ${OSDBASE})
# if [[ "$FS" == "zfs" ]]
# then
#   echo "ceph OSDs don't work well on ZFS, please put OSDBASE on a different file system"
#   exit 1
# fi

# try create the network
OPUT=$(docker network create --subnet ${SUB24}.0/24 ${DOCKERNET} 2>&1)
if [[ $? != 0 ]]
then
  echo "[ABORT] could not create docker subnet"
  echo $OPUT | sed "s/^/[FATAL ERROR] /"
  echo "Perhaps you did not tear down the last devmachine?"
  exit 1
else
  echo "[INFO] docker network created"
fi

#ensure we have the latest containers
echo "[INFO] pulling containers"
docker pull btrdb/cephdaemon 2>&1 | sed "s/^/[INFO][PULL] /"
docker pull btrdb/stubetcd:latest 2>&1 | sed "s/^/[INFO][PULL] /"
docker pull btrdb/${PREFIX}db:${VERSION} 2>&1 | sed "s/^/[INFO][PULL] /"
docker pull btrdb/${PREFIX}apifrontend:${VERSION} 2>&1 | sed "s/^/[INFO][PULL] /"

# all containers are gone, lets create new ones
OPUT=$(docker run -d --net ${DOCKERNET} --ip ${SUB24}.5 \
 --name ${CONTAINER_PREFIX}ceph-mon \
 -v ${OSDBASE}/etc/ceph:/etc/ceph \
 --restart always \
 -v ${OSDBASE}/var/lib/ceph/:/var/lib/ceph/ \
 -e MON_IP=${SUB24}.5 \
 -e CEPH_PUBLIC_NETWORK=${SUB24}.0/24 \
 btrdb/cephdaemon mon 2>&1)

if [[ $? != 0 ]]
then
  echo "[ABORT] Could not start monitor container"
  echo $OPUT | sed "s/^/[FATAL ERROR] /"
  exit 1
fi

echo "[INFO] waiting for monitor container to start"

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
if ! grep -e "name len = 256" ${OSDBASE}/etc/ceph/ceph.conf >/dev/null
then
  echo "[WARN] custom monitor configs and restarting mon"
  echo "osd max object name len = 256" >> ${OSDBASE}/etc/ceph/ceph.conf
  echo "osd max object namespace len = 64" >> ${OSDBASE}/etc/ceph/ceph.conf
  echo "mon allow pool delete = true" >> ${OSDBASE}/etc/ceph/ceph.conf
  docker restart ${CONTAINER_PREFIX}ceph-mon >/dev/null 2>&1
else
  echo "[INFO] custom monitor configs found"
fi

OPUT=$(docker run -d --net ${DOCKERNET} --ip ${SUB24}.4 \
 --name ${CONTAINER_PREFIX}ceph-mgr \
  --restart always \
 -v ${OSDBASE}/etc/ceph:/etc/ceph \
 -v ${OSDBASE}/var/lib/ceph/:/var/lib/ceph/ \
 -e MON_IP=${SUB24}.5 \
 -e CEPH_PUBLIC_NETWORK=${SUB24}.0/24 \
 btrdb/cephdaemon mgr 2>&1)

if [[ $? != 0 ]]
then
  echo "[ABORT] Could not start mgr container"
  echo $OPUT | sed "s/^/[FATAL ERROR] /"
  exit 1
fi

echo "[INFO] ceph MGR started"
#
for osdnum in 0 1 2 3
do
  lastoctet=$(( 10 + $osdnum ))
  PSTORAGEOPT="-v ${OSDBASE}/osd${osdnum}:/var/lib/ceph/osd"
  if [[ "$USE_EPHEMERAL_STORAGE" == "Y" ]]
  then
    PSTORAGEOPT=" "
  fi
  OPUT=$(docker run -d --net ${DOCKERNET} --ip ${SUB24}.${lastoctet} \
    --name ${CONTAINER_PREFIX}ceph-osd-${osdnum} \
    --restart always \
   -v ${OSDBASE}/etc/ceph:/etc/ceph \
   -v ${OSDBASE}/var/lib/ceph/bootstrap-osd:/var/lib/ceph/bootstrap-osd \
   ${PSTORAGEOPT} \
   -e OSD_TYPE=directory \
   btrdb/cephdaemon osd 2>&1)

   if [[ $? != 0 ]]
   then
     echo "[ABORT] Could not start OSD container"
     echo $OPUT | sed "s/^/[FATAL ERROR] /"
     exit 1
   fi
   echo "[INFO] ceph OSD ${osdnum} started"
   sleep 2
done

# start etcd
OPUT=$(docker run -d --net ${DOCKERNET} --ip ${SUB24}.20 \
  --name ${CONTAINER_PREFIX}etcd \
  --restart always \
  -p 2379:2379 \
  -v ${ETCDBASE}/db:/var/lib/etcd \
  -e ETCD_DATA_DIR=/var/lib/etcd \
  -e ETCD_LISTEN_CLIENT_URLS=http://${SUB24}.20:2379 \
  -e ETCD_ADVERTISE_CLIENT_URLS=http://${SUB24}.20:2379 \
  btrdb/stubetcd:latest 2>&1)

if [[ $? != 0 ]]
then
  echo "[ABORT] Could not start etcd container"
  echo $OPUT | sed "s/^/[FATAL ERROR] /"
  exit 1
fi

echo "[INFO] etcd started"

echo "[INFO] checking pools"
# check that the ceph pools are okay
if ! bin/dvceph osd pool get ${HOTPOOL} size >/dev/null 2>&1
then
  # maybe it doesn't exist
  if ! bin/dvceph osd pool create ${HOTPOOL} 16 16  2>&1 | sed "s/^/[INFO][POOL CREATE] /"
  then
    echo "could not create hot pool"
    exit 1
  fi
else
  echo "[INFO] hot pool exists"
fi

if ! bin/dvceph osd pool get ${COLDPOOL} size >/dev/null 2>&1
then
  # maybe it doesn't exist
  if ! bin/dvceph osd pool create ${COLDPOOL} 16 16 2>&1 | sed "s/^/[INFO][POOL CREATE] /"
  then
    echo "could not create cold pool"
    exit 1
  fi
else
  echo "[INFO] cold pool exists"
fi

if ! bin/dvceph osd pool get ${JOURNALPOOL} size >/dev/null 2>&1
then
  # maybe it doesn't exist
  if ! bin/dvceph osd pool create ${JOURNALPOOL} 16 16 2>&1 | sed "s/^/[INFO][POOL CREATE] /"
  then
    echo "could not create journal pool"
    exit 1
  fi
else
  echo "[INFO] journal pool exists"
fi

# run the btrdb ensuredb command
echo "[INFO] checking database is initialized"
docker run -it \
  --net ${DOCKERNET} \
  --ip ${SUB24}.21 \
  -v ${OSDBASE}/etc/ceph:/etc/ceph \
  -e ETCD_ENDPOINT=http://${ETCD_ENDPOINT} \
  -e CEPH_HOT_POOL=${HOTPOOL} \
  -e CEPH_JOURNAL_POOL=${JOURNALPOOL} \
  -e BTRDB_BLOCK_CACHE=62500 \
  -e CEPH_DATA_POOL=${COLDPOOL} \
  -e MY_POD_IP=${SUB24}.21 \
  btrdb/${PREFIX}db:${VERSION} ensuredb | sed "s/^/[INFO][DB INIT] /"

if [[ $? != 0 ]]
then
  echo "[ABORT] Could not initialize DB"
  exit 1
fi

# start the btrdb container
OPUT=$(docker run -d \
  --net ${DOCKERNET} \
  --name ${CONTAINER_PREFIX}btrdbd \
  --ip ${SUB24}.21 \
  --restart always \
  -v ${OSDBASE}/etc/ceph:/etc/ceph \
  -e ETCD_ENDPOINT=http://${ETCD_ENDPOINT} \
  -e CEPH_HOT_POOL=${HOTPOOL} \
  -e CEPH_JOURNAL_POOL=${JOURNALPOOL} \
  -e BTRDB_ENABLE_FAULT_INJECT=YES \
  -e BTRDB_BLOCK_CACHE=62500 \
  -e BTRDB_ENABLE_OBLITERATE=YES \
  -e CEPH_DATA_POOL=${COLDPOOL} \
  -e MY_POD_IP=${SUB24}.21 \
  btrdb/${PREFIX}db:${VERSION})

if [[ $? != 0 ]]
then
  echo "[ABORT] Could not start DB"
  echo $OPUT | sed "s/^/[FATAL ERROR] /"
  exit 1
fi

echo "[INFO] waiting for DB server to start (20s)"
sleep 9
echo "[INFO] waiting for DB server to start (10s)"
sleep 11
echo "[INFO] database server started"

# set up the admin console
# generate host keys
mkdir -p ${OSDBASE}/etc/adminserver
ssh-keygen -f ${OSDBASE}/etc/adminserver/id_rsa -N '' -t rsa >/dev/null 2>&1
ssh-keygen -f ${OSDBASE}/etc/adminserver/id_dsa -N '' -t dsa >/dev/null 2>&1

OPUT=$(docker run -d \
  --name ${CONTAINER_PREFIX}console \
  --net ${DOCKERNET} \
  -p ${CONSOLE_PORT}:2222 \
  -p 2223:2223 \
  -p 2224:2224 \
  --ip ${SUB24}.26 \
  --restart always \
  -v ${OSDBASE}/etc/ceph:/etc/ceph \
  -v ${OSDBASE}/etc/adminserver:/etc/adminserver \
  -e ETCD_ENDPOINT=http://${ETCD_ENDPOINT} \
  -e BTRDB_ENDPOINTS=${SUB24}.21:4410 \
  btrdb/${PREFIX}console:${VERSION})

if [[ $? != 0 ]]
then
  echo "[ABORT] Could not start admin console"
  echo $OPUT | sed "s/^/[FATAL ERROR] /"
  exit 1
fi

echo "[INFO] admin console started"

OPUT=$(docker run -d \
  --name ${CONTAINER_PREFIX}mrplotter \
  --net ${DOCKERNET} \
  -p ${PLOTTER_PORT}:443 \
  --ip ${SUB24}.25 \
  --restart always \
  -e ETCD_ENDPOINT=http://${ETCD_ENDPOINT} \
  -e BTRDB_ENDPOINTS=${SUB24}.21:4410 \
  btrdb/${PREFIX}mrplotter:${VERSION})

if [[ $? != 0 ]]
then
  echo "[ABORT] Could not start plotter"
  echo $OPUT | sed "s/^/[FATAL ERROR] /"
  exit 1
fi

echo "[INFO] plotter server started"

OPUT=$(docker run -d \
  --name ${CONTAINER_PREFIX}apifrontend \
  --net ${DOCKERNET} \
  --restart always \
  -p ${API_GRPC_PORT}:4410 \
  -p ${API_HTTP_PORT}:9000 \
  --ip ${SUB24}.27 \
  -e ETCD_ENDPOINT=http://${ETCD_ENDPOINT} \
  -e BTRDB_ENDPOINTS=${SUB24}.21:4410 \
  btrdb/${PREFIX}apifrontend:${VERSION} 2>&1)

if [[ $? != 0 ]]
then
  echo "[ABORT] Could not start api frontend"
  echo $OPUT | sed "s/^/[FATAL ERROR] /"
  exit 1
fi

echo "[COMPLETE] ========================="
echo "Plotter is on https://127.0.0.1:${PLOTTER_PORT}"
echo "Console is on ssh://127.0.0.1:${CONSOLE_PORT}"
echo "BTrDB GRPC api is on 127.0.0.1:${API_GRPC_PORT}"
echo "BTrDB HTTP api is on http://127.0.0.1:${API_HTTP_PORT}"
echo "BTrDB HTTP swagger UI is on http://127.0.0.1:${API_HTTP_PORT}/swag"
