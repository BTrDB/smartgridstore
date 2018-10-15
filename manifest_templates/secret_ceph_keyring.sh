#!/bin/bash
# {{ .GenLine }}
# this creates the ceph config secret

{{ if .SiteInfo.Ceph.MountConfig }}
# this file does nothing if you are mounting your ceph config directly
{{ else }}
set -ex
if [ ! -e /etc/ceph/ceph.conf ]
then
    echo "Could not locate /etc/ceph/ceph.conf"
    exit 1
fi
if [ ! -e /etc/ceph/ceph.client.admin.keyring ]
then
    echo "Could not locate /etc/ceph/ceph.client.admin.keyring"
    exit 1
fi
pushd /etc/ceph
kubectl create secret -n {{.TargetNamespace}} generic ceph-keyring --from-file=ceph.client.admin.keyring --from-file=ceph.conf
popd

{{ end }}
