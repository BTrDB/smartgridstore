#!/bin/bash

set -e

if [ -z "$CLUSTER_INFO" ]; then
  echo "you need to source cluster-info.sh after you have filled it in"
  exit 1
fi

TOKEN=$(uuidgen)
INITIAL_CLUSTER=""

DIRTY=0
echo -e "\033[34;1mchecking server liveness\033[0m"
# quickly do all the fingerprint accepting
for srv in $CLUSTER_INFO
do
  arr=(${srv//,/ })
  nodename=${arr[0]}
  eip=${arr[1]}
  iip=${arr[2]}
  if [ -n "$INITIAL_CLUSTER" ]; then
    INITIAL_CLUSTER=${INITIAL_CLUSTER},
  fi
  INITIAL_CLUSTER=${INITIAL_CLUSTER}$nodename=http://$iip:2380
  echo -en "\033[34;1m"
  ssh $SSH_USER@$eip echo -e " - $nodename is alive"
  echo -en "\033[0m"
done
echo -e "\033[34;1mchecking servers for updates\033[0m"
# now do the lengthy upgrade
for srv in $CLUSTER_INFO
do
  arr=(${srv//,/ })
  nodename=${arr[0]}
  eip=${arr[1]}
  iip=${arr[2]}
  echo -e "\033[34;1m - updating package lists on $nodename\033[0m"
  UPGR=$(ssh $SSH_USER@$eip 'echo $(sudo apt-get update 2>&1) >/dev/null && /usr/lib/update-notifier/apt-check 2>&1')
  if [[ "$UPGR" != "0;0" ]]; then
    echo -e "\033[34;1m  > $nodename has outstanding updates. Installing and rebooting\033[0m"
    ssh $SSH_USER@$eip "sudo apt-get dist-upgrade -y && sudo reboot" || true
    DIRTY=1
  else
    echo -e "\033[34;1m  > $nodename is up to date\033[0m"
  fi
done

if [[ $DIRTY != 0 ]]; then
  echo -e "\033[34;1msome nodes were rebooted after updates. Wait a bit then rerun this script\033[0m"
  exit 1
fi
echo -e "\033[34;1minstalling docker, etcd and fleet\033[0m"
INITIAL_CLUSTER=$(echo -n $INITIAL_CLUSTER | sed -e 's/[\/&]/\\&/g')
for srv in $CLUSTER_INFO
do
  arr=(${srv//,/ })
  nodename=${arr[0]}
  eip=${arr[1]}
  iip=${arr[2]}
  echo -e "\033[34;1m - installing on $nodename\033[0m"
  ssh -i $IDENTITY $SSH_USER@$eip 'sudo apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D;\
    echo "deb https://apt.dockerproject.org/repo ubuntu-xenial main" | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null ; \
    sudo apt-get update; sudo apt-get install -y linux-image-extra-$(uname -r); sudo apt-get install -y docker-engine; sudo docker pull immesys/btrdb-qs-wb'
  cp templates/etcd.template.service units/etcd.service
  sed -i.bak "s/XX_INTERNAL_IP/$iip/g" units/etcd.service
  sed -i.bak "s/XX_NODENAME/$nodename/g" units/etcd.service
  sed -i.bak "s/XX_INITIAL_CLUSTER/$INITIAL_CLUSTER/g" units/etcd.service
  sed -i.bak "s/XX_TOKEN/$TOKEN/g" units/etcd.service
  scp -i $IDENTITY units/etcd.service $SSH_USER@$eip:/tmp/etcd.service
  ssh -i $IDENTITY $SSH_USER@$eip "sudo mv /tmp/etcd.service /etc/systemd/system/ ; sudo systemctl daemon-reload ; \
    sudo systemctl enable etcd.service ; sudo systemctl start etcd.service"

  ssh -i $IDENTITY $SSH_USER@$eip "sudo systemctl stop fleet; curl -s -L https://github.com/coreos/fleet/releases/download/v0.11.7/fleet-v0.11.7-linux-amd64.tar.gz > /tmp/fleet.tgz ; \
    cd /tmp ; tar -xf /tmp/fleet.tgz ; sudo cp fleet-v0.11.7-linux-amd64/f* /usr/bin/"
  cp templates/fleet.template.service units/fleet.service
  sed -i.bak "s/XX_NODENAME/$nodename/g" units/fleet.service
  scp -i $IDENTITY units/fleet.service $SSH_USER@$eip:/tmp/fleet.service
  scp -i $IDENTITY templates/fleet.template.socket $SSH_USER@$eip:/tmp/fleet.socket
  ssh -i $IDENTITY $SSH_USER@$eip "sudo mv /tmp/fleet.service /etc/systemd/system/ ; \
    sudo mv /tmp/fleet.socket /etc/systemd/system/ ; sudo systemctl daemon-reload ; \
    sudo systemctl enable fleet.service ; sudo systemctl start fleet.service"
done
echo -e "\033[34;1mserver preparation complete\033[0m"
