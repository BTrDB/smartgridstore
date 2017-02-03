#!/bin/bash
pushd $(mktemp -d)
ssh-keygen -t rsa -N "" -f id_rsa
kubectl create secret generic admin-host-key --from-file id_rsa
popd
