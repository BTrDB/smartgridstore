#!/bin/bash

if [[ $1 = "backup" ]]
then
  : ${BACKUP_INTERVAL_SECS:=3600}
  set -ex
  echo "running etcd backup pod"
  while true
  do
    sleep $(( BACKUP_INTERVAL_SECS ))
    ETCDCTL_API=3 /bin/etcdctl --endpoints "http://etcd:2379" snapshot save /srv/persist/snap.db
    savelog /srv/persist/snap.db
  done
fi

if [[ -e /srv/persist/create_timestamp ]]
then
  echo "Detected this is a reboot/migrate of the pod. Forcing ETCD_INITIAL_CLUSTER_STATE to 'existing'"
  export ETCD_INITIAL_CLUSTER_STATE=existing
else
  echo "Detected this is a brand new etcd pod. Forcing ETCD_INITIAL_CLUSTER_STATE to 'new'"
  export ETCD_INITIAL_CLUSTER_STATE=new
  echo `date` > /srv/persist/create_timestamp
fi

/bin/etcd --name $MY_POD_NAME \
 --initial-advertise-peer-urls http://${MY_POD_NAME}.etcd:2380 \
 --advertise-client-urls http://${MY_POD_NAME}.etcd:2379
