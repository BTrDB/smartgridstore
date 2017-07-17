#!/bin/bash
# {{ .GenLine }}
#this creates a new host key for the console and puts it in as a secret
set -ex
pushd $(mktemp -d)
ssh-keygen -t rsa -N "" -f id_rsa
kubectl create secret generic -n {{.TargetNamespace}} admin-host-key --from-file id_rsa
popd
