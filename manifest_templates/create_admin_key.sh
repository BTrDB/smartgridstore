#!/bin/bash
# {{ .GenLine }}
#this creates a new host key for the console and puts it in as a secret
pushd $(mktemp -d)
ssh-keygen -t rsa -N "" -f id_rsa
kubectl create secret generic admin-host-key --from-file id_rsa
popd
