# BTrDB backup tool

This is a tool for making backups of BTrDB clusters, either the etcd data, the ceph data or both. The backups are encrypted using a symmetric key generated from a passphrase, so the backups can be stored safely on insecure offsite storage.

## Creating a backup

You can backup just the metadata stored in etcd (which is small), the raw data stored in ceph (very large) or both. For most use cases we recommend that you use ceph snapshots for normal backups of ceph, and perform regular backups of just etcd using this tool. We only recommend performing a ceph backup using this tool when you are wanting to store a copy off site or are migrating to a different cluster.

### Backing up just etcd

You need to pass the client address of etcd to the tool. This address can be found from the `etcd` service in k8s. You can pass the name of the directory that will be created to store the backup. It is preferable to shut down BTrDB and wait 30 seconds before performing an etcd backup (see the last section of this guide for details).

For example:

```
./backuptool create --outputdir my_backup --etcd 10.107.215.167:2379 --skip-ceph
```

You will need to enter the encryption passphrase (pick something suitably strong). Then you should see something like this:

```
please enter the encryption passphrase: **********
same again: **********
generating (enc/dec)ryption keys from passphrase...
done 521.19675ms
skipping incremental metadata
skipping ceph backup
creating new file ARCHIVE00000.bin
ETCD DONE; included 62 etcd records
```

You can now move the `my_backup` directory somewhere safe.

### Backing up both etcd and ceph

The process is much the same as above, but you should specify the ceph pools to back up. If omitted, all pools will be backed up.

```
./backuptool create --cephconf /etc/ceph/ceph.conf --outputdir my_backup --include-pool btrdbhot --include-pool btrdbcold --etcd 10.107.215.167:2379
```


You should see something like this:

```
please enter the encryption passphrase: ********
same again: ********
generating (enc/dec)ryption keys from passphrase...
done 518.550988ms
skipping incremental metadata
this backup will include 2 pools:
 - btrdbhot
 - btrdbcold
creating new file ARCHIVE00000.bin
CEPH DONE; included 18 ceph objects (skipped 0 unchanged)
ETCD DONE; included 62 etcd records
```

## Performing an incremental backup

Incremental backups allow you to backup only the ceph objects that have changed since a previous full ceph backup. Do not do an incremental backup against an incremental backup. This form of backup is useful for splitting backups into an on-line and off-line phase. First perform a full backup while your cluster is operational and receiving data. That backup is not guaranteed to be consistent (it might be corrupt if restored alone). Then shut down BTrDB and perform an incremental backup. This will be much faster. To restore, first process the full backup, and then the incremental backup.

```
/backuptool create --cephconf /etc/ceph/ceph.conf --outputdir incremental --include-pool btrdbhot --include-pool btrdbcold --etcd 127.0.0.1:2379 --incremental-over my_backup
```

You will need the same encryption phrase as before:

```
please enter the encryption passphrase: ********
same again: ********
generating (enc/dec)ryption keys from passphrase...
done 519.9547ms
this backup will include 2 pools:
 - btrdbhot
 - btrdbcold
creating new file ARCHIVE00000.bin
CEPH DONE; included 125 ceph objects (skipped 17 unchanged)
ETCD DONE; included 483 etcd records
```

## Verifying a backup

To verify that a backup does not have any file corruption, you can inspect it:

```
./backuptool inspect --outputdir my_backup
```

You should get something like this:

```
please enter the encryption passphrase: **********
same again: **********
generating (enc/dec)ryption keys from passphrase...
done 513.49648ms
archive contains 18 ceph objects and 62 etcd records
the following 2 pools are referenced:
 - btrdbhot
 - btrdbcold
```

## Preparing for a backup restore

Before restoring a backup, you should clear the restore target to ensure there are no
left over keys (in either ceph or etcd). This does not apply if you are performing a restore of an incremental backup and you have just done the full backup restore.

To clear etcd:

```
./backuptool clear --etcd 127.0.0.1:2379 --skip-ceph
```

You should see:

```
deleted etcd keys
done
```

To clear ceph:

```
./backuptool clear --cephconf /etc/ceph/ceph.conf --include-pool btrdbhot --include-pool btrdbcold --skip-etcd
```

You should see something like this:

```
deleted ceph object "btrdbhot"/"meta8c9359bab9ae40c483f26f2e08ac656e"
deleted ceph object "btrdbhot"/"meta86d7da1970dc4310944134132e6197d8"

 ... snipped ...

deleted ceph object "btrdbcold"/"555ab4f9e4c74247bada5a817a1341470000001008"
done
```

## Restoring a backup

When restoring a backup, you can pick which of etcd or ceph or both is restored.

To restore etcd (note this can be restored to any etcd server):

```
./backuptool restore --etcd 127.0.0.1:2379 --skip-ceph --outputdir my_backup
```

You should see something like this:

```
please enter the encryption passphrase: *****
same again: *****
generating (enc/dec)ryption keys from passphrase...
done 518.913021ms
connecting etcd
Restore complete: restored 0 ceph objects, skipped 0 ceph objects and restored  62 etcd records
```

To restore ceph, you must specify not only which pools are restored but also what the new name of the pool must be. Note that unlike when creating a backup, if you omit the pools the default is to NOT restore any pools rather than to restore ALL pools. The tool does not create pools for you, so the ceph pools must already exist.

```
./backuptool restore --cephconf /etc/ceph/ceph.conf --outputdir my_backup --restore-pool btrdbhot:restore_hot --restore-pool btrdbcold:restore_cold --skip-etcd
```

You should see something like this:

```
please enter the encryption passphrase: *****
same again: *****
generating (enc/dec)ryption keys from passphrase...
done 508.74391ms
connecting ceph
Restore complete: restored 18 ceph objects, skipped 0 ceph objects and restored  0 etcd records
```

Restoring an incremental backup is just like restoring a normal backup, and is performed after you have done the restore of the full backup, e.g:

```
./backuptool restore --cephconf /etc/ceph/ceph.conf --outputdir incremental --restore-pool btrdbhot:restore_hot --restore-pool btrdbcold:restore_cold --skip-etcd
```

Which gives:

```
please enter the encryption passphrase: *****
same again: *****
generating (enc/dec)ryption keys from passphrase...
done 509.457573ms
connecting ceph
Restore complete: restored 125 ceph objects, skipped 0 ceph objects and restored  0 etcd records
```

## Caveats and known issues

If you perform an etcd backup within 30 seconds of shutting down BTrDB or while BTrDB is running, you will backup some node state data that will interfere with the operation of the restored cluster. To fix this, after performing the etcd restore, while BTrDB is not running, log in to the admin console and under the `btrdb` section, perform an `autoprune`. Then you can start BTrDB and it will work as expected.
