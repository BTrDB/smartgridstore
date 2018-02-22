# ordering

The full documentation can be found on smartgrid.store, but here is the order
you should create things:

### global
If this is the first time installing BTrDB on this k8s cluster (as opposed to
having one already in a different namespace) then create global first
```
global/etcd.clusterrole.yaml
```

### core

Create the secrets first (you must also have done the PV provider). You need to
have a storageclass called `etcd-backup-gce-pd` for the etcd operator to work.

Secrets:
```
core/secret_ceph_keyring.sh
core/create_admin_key.sh
```
Then do etcd:
```
core/etcd.clusterrolebinding.yaml
core/etcd.serviceaccount.yaml
core/etcd-operator.deployment.yaml
core/etcd.cluster.yaml
```
Then wait for your three etcd pods to be up and running ok and do createdb:

```
core/ensuredb.job.yaml
```
Wait for that to finish okay and then do btrdb

```
core/btrdb.statefulset.yaml
```

Wait for that to be running and do the rest:

```
core/adminconsole.deployment.yaml
core/mrplotter.deployment/yaml
```

### ingress

you can do the ingress at any stage after the core has been created
