#!/bin/bash

if [ -z "$CLUSTER_INFO" ]; then
  echo "you need to source cluster-info.sh after you have filled it in"
  exit 1
fi

if [[ -z "$DOMAIN" ]]; then
  echo "you need to set $DOMAIN"
  exit 1
fi

ssh -t $SSH_USER@$DOMAIN "sudo docker run -it -e CLUSTER_INFO=\"$CLUSTER_INFO\" -e DOMAIN=\"$DOMAIN\" \
  /usr/bin/docker run \
  -m 6G \
  -v /srv/mrplotter:/etc/mrplotter \
  -v /srv/certs:/etc/letsencrypt \
  immesys/mrplotter renewcert"
