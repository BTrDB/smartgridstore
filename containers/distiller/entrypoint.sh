#!/bin/bash

if [ -z "$SOURCECODE" ]
then
  echo "You need SOURCECODE set"
  exit 1
fi

set -ex
go get -d -u $SOURCECODE
go install $SOURCECODE
$GOPATH/bin/$(basename $SOURCECODE) | pp
