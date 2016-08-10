#!/bin/bash

if [ -z "$CLUSTER_INFO" ]; then
  echo "you need to source cluster-info.sh after you have filled it in"
  exit 1
fi

echo -e "\033[34;1mrebooting servers\033[0m"
for srv in $CLUSTER_INFO
do
  arr=(${srv//,/ })
  nodename=${arr[0]}
  eip=${arr[1]}
  iip=${arr[2]}
  echo -e "\033[34;1m - rebooting $nodename\033[0m"
  ssh $SSH_USER@$eip sudo reboot
done
echo -e "\033[34;1mdone\033[0m"
