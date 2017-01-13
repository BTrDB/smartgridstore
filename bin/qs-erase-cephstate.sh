#!/bin/bash

set -ex
if [ -z "$CLUSTER_INFO" ]; then
  echo "you need to source cluster-info.sh after you have filled it in"
  exit 1
fi

  for srv in $CLUSTER_INFO
  do
    arr=(${srv//,/ })
    nodename=${arr[0]}
    eip=${arr[1]}
    iip=${arr[2]}
    ssh $SSH_USER@$eip sudo rm -rf /srv/ceph
  done

  ssh -t $SSH_USER@$TUNNEL_IP "sudo docker run -it -e KV_PORT=2379 \
   -e CLUSTER_INFO=\"$CLUSTER_INFO\" \
   immesys/btrdb-qs-wb etcdctl rm --recursive ceph-config"
