#!/bin/bash
FS="\033[34;1m"
FE="\033[0m"
if [ -z "$CLUSTER_INFO" ]; then
  echo -e "${FS}you need to source cluster-info.sh after you have filled it in${FE}"
  exit 1
fi

bin/fleetctl --tunnel=$TUNNEL_IP --ssh-username=$SSH_USER $@
