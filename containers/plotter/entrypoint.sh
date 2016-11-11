#!/bin/bash
set -xe


if [[ $1 == "renewcert" ]]
then
  letsencrypt renew
exit 0
fi
if [[ $1 == "createcert" ]]
then
  letsencrypt certonly --standalone -d $DOMAIN
  exit 0
fi
if [[ $1 == "shell" ]]
then
  set +xe
  bash -i
exit 0
fi
if [[ $1 == "metadata" ]]
then
  mkdir -p /etc/mrplotter
  if [ ! -e /etc/mrplotter/tagconfig.json ]
  then 
  cat >/etc/mrplotter/tagconfig.json <<EOFF
  {
      "public": ["/"]
  }
EOFF
  fi

  ls /etc/mrplotter
  cat /etc/mrplotter/tagconfig.json
  /srv/go/src/github.com/SoftwareDefinedBuildings/mr-plotter/tools/metadata.py $MONGO_IP 27017 /etc/mrplotter/tagconfig.json
exit 0
fi

if [[ $1 == "accounts" ]]
then
  /srv/go/src/github.com/SoftwareDefinedBuildings/mr-plotter/tools/accounts.py $MONGO_IP
exit 0
fi


if [ -z "$DOMAIN" ]
then
  echo "you need to set DOMAIN"
  exit 0
fi

if [ -z "$BTRDB_IP" ]
then
  echo "you need to set BTRDB_IP"
  exit 0
fi

if [ -z "$METADATA_IP" ]
then
  echo "you need to set METADATA_IP"
  exit 0
fi

if [ ! -f /etc/mrplotter/plotter.ini ]; then

cat >/etc/mrplotter/plotter.ini <<EOFF
http_port=80
https_port=443
use_http=true
use_https=true
https_redirect=true
plotter_dir=/srv/go/src/github.com/SoftwareDefinedBuildings/mr-plotter/assets
cert_file=/etc/letsencrypt/live/${DOMAIN}/fullchain.pem
key_file=/etc/letsencrypt/live/${DOMAIN}/privkey.pem

db_addr=${BTRDB_IP}:4410
num_data_conn=3
num_bracket_conn=2
max_data_requests=8
max_bracket_requests=8
metadata_server=http://${METADATA_IP}:4523
mongo_server=${MONGO_IP}:27017
csv_url=http://${BTRDB_IP}:9000/directcsv

session_expiry_seconds=604800 # 1 week
session_purge_interval_seconds=14400 # 6 hours
csv_max_points_per_stream=100000
outstanding_request_log_interval=30
num_goroutines_log_interval=10
db_data_timeout_seconds=10
db_bracket_timeout_seconds=10
EOFF


fi

mr-plotter /etc/mrplotter/plotter.ini |& pp
