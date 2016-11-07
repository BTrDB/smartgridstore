#!/bin/bash

if [ -z "$CLUSTER_INFO" ]; then
  echo "you need to source cluster-info.sh after you have filled it in"
  exit 1
fi

echo -e "\033[34;1mupdating docker images\033[0m"
for srv in $CLUSTER_INFO
do
  arr=(${srv//,/ })
  nodename=${arr[0]}
  eip=${arr[1]}
  iip=${arr[2]}
  echo -e "\033[34;1m - updating $nodename\033[0m"

  ssh $SSH_USER@$eip sudo docker pull immesys/mrplotter
  ssh $SSH_USER@$eip sudo docker pull immesys/btrdb-ceph-3.4
  ssh $SSH_USER@$eip sudo docker pull immesys/btrdb-qs-receiver
  ssh $SSH_USER@$eip sudo docker pull immesys/btrdb-qs-sync
  ssh $SSH_USER@$eip sudo docker pull immesys/btrdb-qs-wb

  #ssh $SSH_USER@$eip sudo docker pull btrdb/notebook:1.0
done
echo -e "\033[34;1mdone\033[0m"
