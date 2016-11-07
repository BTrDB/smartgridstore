#!/bin/bash

FS="\033[34;1m"
FE="\033[0m"
if [ -z "$CLUSTER_INFO" ]; then
  echo -e "${FS}you need to source cluster-info.sh after you have filled it in${FE}"
  exit 1
fi

: ${CEPH_CLUSTER:=ceph}

osd_idx=0

CFG="qs-config.sh"

if [ -n "$(ls -A units/*)" ]
then
  if [ -z "$EXPERT_MODE" ]
  then
    echo "There are files in units/ and EXPERT_MODE is unset"
    echo "Please consult the guide"
    exit 1
  else
    echo "There are files in units/ but EXPERT_MODE is set"
    echo "continuing"
  fi
fi

rm -f units/*.service
rm -f units/*.bak

function GEN_MON {
  nodename="$1"
  iip="$2"
  cp templates/ceph-mon.template.service units/ceph-mon-$nodename.service
  sed -i.bak "s/XX_INTERNAL_IP/$iip/g" units/ceph-mon-$nodename.service
  sed -i.bak "s/XX_CLUSTER/$CEPH_CLUSTER/g" units/ceph-mon-$nodename.service
  sed -i.bak "s/XX_NODENAME/$nodename/g" units/ceph-mon-$nodename.service
  rm -f units/*.bak
}

function GEN_OSD {
  echo -e "${FS}skipping GEN_OSD (will run on next pass)${FE}"
}
function FORMAT_DISK {
  echo -e "${FS}skipping FORMAT_DISK (will run on next pass)${FE}"
}
function GEN_BTRDB {
  echo -e "${FS}skipping GEN_BTRDB (will run on next pass)${FE}"
}
function GEN_MONGODB {
  echo -e "${FS}skipping GEN_MONGODB (will run on next pass)${FE}"
}
function GEN_RECEIVER {
  echo -e "${FS}skipping GEN_RECEIVER (will run on next pass)${FE}"
}
function GEN_SYNC2Q {
  echo -e "${FS}skipping GEN_SYNC2Q (will run on next pass)${FE}"
}
function FORMAT_BTRDB {
  echo -e "${FS}skipping FORMAT_BTRDB (will run on next pass)${FE}"
}
function CREATE_CEPH_POOL {
  echo -e "${FS}skipping CREATE_CEPH_POOL (will run on next pass)${FE}"
}
function GEN_METADATA {
  echo -e "${FS}skipping GEN_METADATA (will run on next pass)${FE}"
}
function GEN_PLOTTER {
  echo -e "${FS}skipping GEN_PLOTTER (will run on next pass)${FE}"
}
function REMOVE_AFTER_CHECKING_FILE {
  echo -e "${FS}You need to read and edit $CFG${FE}"
  echo -e "${FS}By default it will format every disk on every server${FE}"
  echo -e "${FS}Including the OS disks${FE}"
  exit 1
}
function GEN_SSL_CERT {
  echo -e "${FS}skipping GEN_SSL_CERT (will run on next pass)${FE}"
}
source $CFG

for unit in $(ls units/ceph-mon-*.service)
do
  echo -e "${FS}Loading unit $unit${FE}"
  bin/fleetctl --tunnel $TUNNEL_IP --ssh-username $SSH_USER start $unit
done

function GEN_MON {
  echo -e "${FS}skipping mon (done already)${FE}"
}

function GEN_OSD {
  nodename="$1"
  iip="$2"
  drive="$3"
  shift 3
  crush="$*"
  if [ -z "$crush" ]; then
    crush="root=default host=$nodename"
  fi
  drive=$(echo -n $drive | sed -e 's/[\/&]/\\&/g')
  unit=units/ceph-osd-$nodename-$(printf %02d $osd_idx).service
  osd_idx=$(($osd_idx+1))
  cp templates/ceph-osd.template.service $unit
  sed -i.bak "s/XX_DEVICE/$drive/g" $unit
  sed -i.bak "s/XX_CLUSTER/$CEPH_CLUSTER/g" $unit
  sed -i.bak "s/XX_NODENAME/$nodename/g" $unit
  sed -i.bak "s/XX_CRUSHLOC/$crush/g" $unit
  rm -f units/*.bak
}

function GEN_BTRDB {
  nodename="$1"
  iip="$2"
  unit=units/btrdb-$nodename.service
  cp templates/btrdb.template.service $unit
  sed -i.bak "s/XX_NODENAME/$nodename/g" $unit
  sed -i.bak "s/XX_INTERNAL_IP/$iip/g" $unit
  rm -f units/*.bak
}
function GEN_MONGODB {
  nodename="$1"
  unit=units/mongo-$nodename.service
  cp templates/mongo.template.service $unit
  sed -i.bak "s/XX_NODENAME/$nodename/g" $unit
  rm -f units/*.bak
}
function FORMAT_DISK {
  nodename="$1"
  eip="$2"
  drive="$3"
  ssh -t $SSH_USER@$eip sudo /usr/bin/docker run -it \
    --net=host \
    --privileged=true \
    --pid=host \
    -v /dev/:/dev/ \
    -v /srv/ceph:/var/lib/ceph \
    -e OSD_DEVICE=$drive \
    -e CLUSTER=$CEPH_CLUSTER \
    -e OSD_TYPE=prepare \
    -e OSD_FORCE_ZAP=1 \
    -e KV_TYPE=etcd \
    -e KV_IP=127.0.0.1 \
    -e KV_PORT=2379 \
    ceph/daemon:tag-build-master-jewel-ubuntu-16.04 osd
}

function GEN_SSL_CERT {
  echo -e "${FS}generating ssl cert for $1 ${FE}"
  ssh -t $SSH_USER@$1 "sudo /usr/bin/docker run -it -e DOMAIN=$1 -p 443:443 -p 80:80 \
      -v /srv/certs:/etc/letsencrypt immesys/mrplotter createcert"
}
function GEN_METADATA {
  echo -e "${FS}generating plotter-metadata for $1 ${FE}"
  nodename="$1"
  mongo="$2"
  unit=units/plotter-metadata-$nodename.service
  cp templates/plotter-metadata.template.service $unit
  sed -i.bak "s/XX_NODENAME/$nodename/g" $unit
  sed -i.bak "s/XX_MONGO_IP/$mongo/g" $unit
  rm -f units/*.bak
}
function GEN_PLOTTER {
  echo -e "${FS}generating plotter for $1 ${FE}"
  nodename="$1"
  domain="$2"
  metadata="$3"
  btrdb="$4"
  mongo="$5"
  unit=units/plotter-$nodename.service
  cp templates/plotter.template.service $unit
  sed -i.bak "s/XX_NODENAME/$nodename/g" $unit
  sed -i.bak "s/XX_MONGO_IP/$mongo/g" $unit
  sed -i.bak "s/XX_BTRDB_IP/$btrdb/g" $unit
  sed -i.bak "s/XX_METADATA_IP/$metadata/g" $unit
  sed -i.bak "s/XX_DOMAIN/$domain/g" $unit
  rm -f units/*.bak
}
function GEN_RECEIVER {
  echo -e "${FS}generating receiver for $1 ${FE}"
  nodename="$1"
  mongo="$2"
  unit=units/receiver-$nodename.service
  cp templates/receiver.template.service $unit
  sed -i.bak "s/XX_NODENAME/$nodename/g" $unit
  sed -i.bak "s/XX_MONGO/$mongo/g" $unit
  rm -f units/*.bak
}
function GEN_SYNC2Q {
  echo -e "${FS}generating sync2q for $1 ${FE}"
  nodename="$1"
  mongo="$2"
  btrdb="$3"
  unit=units/sync2q-$nodename.service
  cp templates/sync2q.template.service $unit
  sed -i.bak "s/XX_NODENAME/$nodename/g" $unit
  sed -i.bak "s/XX_MONGO/$mongo/g" $unit
  sed -i.bak "s/XX_BTRDB/$btrdb/g" $unit
  rm -f units/*.bak
}
source $CFG
rm -f units/*.bak
ii=0
while [ $ii -lt $osd_idx ]; do
  unit="units/ceph-osd-*-$(printf %02d $ii).service"
  bin/fleetctl --tunnel $TUNNEL_IP --ssh-username $SSH_USER start $unit
  echo -e "${FS}Waiting for OSD to stabilize${FE}"
  sleep 10
  ii=$((ii+1))
done
for unit in units/mongo-*
do
  echo -e "${FS}starting $unit ${FE}"
  bin/fleetctl --tunnel $TUNNEL_IP --ssh-username $SSH_USER start $unit
done

function GEN_OSD {
  echo -e "${FS}skipping GEN_OSD (done already)${FE}"
}
function FORMAT_DISK {
  echo -e "${FS}skipping FORMAT_DISK (done already)${FE}"
}
function GEN_BTRDB {
  echo -e "${FS}skipping GEN_BTRDB (done already)${FE}"
}
function GEN_MONGODB {
  echo -e "${FS}skipping GEN_MONGODB (done already)${FE}"
}
function GEN_RECEIVER {
  echo -e "${FS}skipping GEN_RECEIVER (done already)${FE}"
}
function GEN_SYNC2Q {
  echo -e "${FS}skipping GEN_SYNC2Q (done already)${FE}"
}
function CREATE_CEPH_POOL {
  poolname="$1"
  args="$@"
  ssh -t $SSH_USER@$TUNNEL_IP "sudo /usr/bin/docker run -it -e KV_PORT=2379 \
    immesys/btrdb-qs-wb ceph osd pool delete $poolname $poolname --yes-i-really-really-mean-it" || true
  ssh -t $SSH_USER@$TUNNEL_IP "sudo /usr/bin/docker run -it -e KV_PORT=2379 \
    immesys/btrdb-qs-wb ceph osd pool create $@"
}

function GEN_METADATA {
  echo -e "${FS}skipping GEN_METADATA (done already)${FE}"
}
function GEN_PLOTTER {
  echo -e "${FS}skipping GEN_PLOTTER (done already)${FE}"
}
function GEN_SSL_CERT {
  echo -e "${FS}skipping GEN_SSL_CERT (done already)${FE}"
}

source $CFG

function FORMAT_BTRDB {
  nodename="$1"
  eip="$2"
  mongo="$3"
  ssh -t $SSH_USER@$eip sudo /usr/bin/docker run -it \
    -v /srv/btrdb:/etc/btrdb  \
    -p 4410:4410 \
    -p 9000:9000 \
    -e KV_PORT=2379 \
    -e BTRDB_MONGO_SERVER=$mongo:27017 \
    -e BTRDB_STORAGE_PROVIDER=ceph \
    immesys/btrdb-ceph-3.4 makedb
}

function CREATE_CEPH_POOL {
  echo -e "${FS}skipping CREATE_CEPH_POOL(done already)${FE}"
}

source $CFG

for unit in units/btrdb-*
do
  echo -e "${FS}starting $unit ${FE}"
  bin/fleetctl --tunnel $TUNNEL_IP --ssh-username $SSH_USER start $unit
done

for unit in units/plotter-*
do
  echo -e "${FS}starting $unit ${FE}"
  bin/fleetctl --tunnel $TUNNEL_IP --ssh-username $SSH_USER start $unit
done
for unit in units/receiver-*
do
  echo -e "${FS}starting $unit ${FE}"
  bin/fleetctl --tunnel $TUNNEL_IP --ssh-username $SSH_USER start $unit
done
for unit in units/sync2q-*
do
  echo -e "${FS}starting $unit ${FE}"
  bin/fleetctl --tunnel $TUNNEL_IP --ssh-username $SSH_USER start $unit
done
