#!/bin/bash

if [ -z "$CLUSTER_INFO" ]; then
  echo "you need to source cluster-info.sh after you have filled it in"
  exit 1
fi


ssh -t $SSH_USER@$TUNNEL_IP "sudo docker pull immesys/btrdb-qs-wb && sudo docker run -it -v /srv/mrplotter:/etc/mrplotter -v /srv/sync:/etc/sync -v /srv/certs:/etc/letsencrypt -e KV_PORT=2379 -e CLUSTER_INFO=\"$CLUSTER_INFO\" immesys/btrdb-qs-wb $@"
