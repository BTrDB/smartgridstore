#!/bin/bash

set -xe
echo ytagbase="$YTAG" > syncconfig.ini
if [[ ! -e /etc/sync/upmuconfig.ini ]]
then
  touch /etc/sync/upmuconfig.ini
fi
ln -s /etc/sync/upmuconfig.ini upmuconfig.ini
sync2_quasar |& pp
