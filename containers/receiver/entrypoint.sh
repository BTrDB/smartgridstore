#!/bin/bash

: ${CLUSTER:=ceph}
: ${CEPH_CLUSTER_NETWORK:=${CEPH_PUBLIC_NETWORK}}
: ${CEPH_DAEMON:=${1}} # default daemon to first argument
: ${CEPH_GET_ADMIN_KEY:=0}
: ${HOSTNAME:=$(hostname -s)}
: ${MON_NAME:=${HOSTNAME}}
: ${NETWORK_AUTO_DETECT:=0}
: ${MDS_NAME:=mds-${HOSTNAME}}
: ${OSD_FORCE_ZAP:=0}
: ${OSD_JOURNAL_SIZE:=100}
: ${CRUSH_LOCATION:=root=default host=${HOSTNAME}}
: ${KV_TYPE:=etcd} # valid options: consul, etcd or none
: ${KV_PORT:=4001} # PORT 8500 for Consul
: ${CLUSTER_PATH:=ceph-config/${CLUSTER}}
export KV_IP=$(netstat -nr | grep '^0\.0\.0\.0' | awk '{print $2}')

function log {
  if [ -z "$*" ]; then
    return 1
  fi

  TIMESTAMP=$(date '+%F %T')
  echo "${TIMESTAMP}  $0: $*"
  return 0
}

#inherited from ceph container
source /config.kv.sh

# pull config and ceph key
get_config
get_admin_key

receiver |& pp
