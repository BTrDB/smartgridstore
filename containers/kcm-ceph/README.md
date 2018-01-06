# Kube-Controller-Manager replacement

In their infinite wisdom, upstream did not package rbd into the default image
that kube-controller-manager runs in. This dockerfile builds a replacement image
that is bigger, but can also do rbd pv's and pvc.

To use this, you need to change the image used for kcm. For the kubeadm install
this appears to be in /etc/kubernetes/manifests/kube-controller-manager.json or
something like that. Change it to btrdb/kubernetes-controller-manager-rbd:1.5.2
and restart the master

My experience was that certain daemon pods died when you do this, but deleting
them and letting the ds recreate them fixed it all.
