#!/bin/bash

set -xe
echo ytagbase="$YTAG" > syncconfig.ini
ln -s
sync2_quasar |& pp
