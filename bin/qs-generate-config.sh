#!/bin/bash

set -e

if [ -z "$CLUSTER_INFO" ]; then
  echo "you need to source cluster-info.sh after you have filled it in"
  exit 1
fi

echo -e "\033[34;1mchecking server liveness\033[0m"
for srv in $CLUSTER_INFO
do
  arr=(${srv//,/ })
  nodename=${arr[0]}
  eip=${arr[1]}
  iip=${arr[2]}
  echo -en "\033[34;1m"
  ssh $SSH_USER@$eip echo -e " - $nodename is alive"
  echo -en "\033[0m"
done

CFG="qs-config.sh"

DONE_APPS=0
echo "# Comment out the lines for drives that you do not want to" > $CFG
echo "# format, generate OSD units for. You can also comment out" >> $CFG
echo "# the generation of MON units, as long as you have at least 1" >> $CFG
echo "# after you finish editing the file, remove this line: " >> $CFG
echo "REMOVE_AFTER_CHECKING_FILE" >> $CFG
echo "" >> $CFG
echo -e "\033[34;1mscanning for disks\033[0m"

echo "# create a ceph pool (destroying it if it exists) (args: poolname, pg, pgp, ruleset)" >> $CFG
echo "CREATE_CEPH_POOL btrdb 512 512 replicated" >> $CFG

for srv in $CLUSTER_INFO
do
  arr=(${srv//,/ })
  nodename=${arr[0]}
  eip=${arr[1]}
  iip=${arr[2]}
  echo "#######" >> $CFG
  echo "## node=$nodename external_ip=$eip internal_ip=$iip" >> $CFG
  echo "  # this node will be a Ceph MON" >> $CFG
  echo "  GEN_MON $nodename $iip" >> $CFG
  if [[ $DONE_APPS == 0 ]]; then
    echo "  # install mongo on this node (args: nodename)" >> $CFG
    echo "  GEN_MONGODB $nodename" >> $CFG
    echo "  # install btrdb on this node (args: nodename, mongo)" >> $CFG
    echo "  GEN_BTRDB $nodename $iip" >> $CFG
    echo "  # format the database (args: nodename, externalip, mongo)" >> $CFG
    echo "  FORMAT_BTRDB $nodename $eip $iip" >> $CFG
    echo "  # install the metadata shim (args: nodename, mongo)" >> $CFG
    echo "  GEN_METADATA $nodename $iip" >> $CFG
    echo "  # install the upmu data receiver daemon (args: nodename, mongo, btrdb)" >> $CFG
    echo "  GEN_RECEIVER $nodename $iip $iip" >> $CFG
    echo "  # install the upmu data loader (args: nodename, mongo, btrdb)" >> $CFG
    echo "  #GEN_SYNC2Q $nodename $iip $iip" >> $CFG
    echo "  # generate an SSL certificate for the plotter (args: domain)" >> $CFG
    echo "  GEN_SSL_CERT $DOMAIN" >> $CFG
    echo "  # install the plotter (args: nodename, domain, metadata, btrdb, mongo)" >> $CFG
    echo "  GEN_PLOTTER $nodename $DOMAIN $iip $iip $iip" >> $CFG
    echo "" >> $CFG
    DONE_APPS=1
  fi
  readarray drives < <(ssh -i $IDENTITY $SSH_USER@$eip sudo lsblk -anr --output TYPE,NAME,SIZE,MODEL,SERIAL,WWN)
  idx=0
  for dd in "${drives[@]}"
  do
    fields=($dd)
    if [ ${fields[0]} == "disk" ]; then
      echo "  # drive $idx name=${fields[1]} size=${fields[2]} model=${fields[3]} serial=${fields[4]} wwn=${fields[5]}" >> $CFG
      echo "  FORMAT_DISK $nodename $eip /dev/${fields[1]}" >> $CFG
      echo "  GEN_OSD $nodename $iip /dev/${fields[1]} root=default host=$nodename" >> $CFG
      if [ $( ssh -i $IDENTITY $SSH_USER@$eip sudo find -L /dev/disk/by-id -samefile /dev/${fields[1]} 2>/dev/null | wc -l) -gt 0 ]
      then
        echo "  #persistent options (see guide): " >> $CFG
        for opt in $( ssh -i $IDENTITY $SSH_USER@$eip sudo find -L /dev/disk/by-id -samefile /dev/${fields[1]} 2>/dev/null )
        do
          echo "  #GEN_OSD $nodename $iip $opt root=default host=$nodename" >> $CFG
        done
        for opt in $( ssh -i $IDENTITY $SSH_USER@$eip sudo find -L /dev/disk/by-path -samefile /dev/${fields[1]} 2>/dev/null )
        do
          echo "  #GEN_OSD $nodename $iip $opt root=default host=$nodename" >> $CFG
        done
      fi
      idx=$(($idx+1))
      echo "" >> $CFG
      echo -e "\033[34;1m - found $nodename::${fields[1]} \033[0m"
    fi
  done
  echo "" >> $CFG
done
