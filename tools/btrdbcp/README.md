# BTRDBCP

This is a utility for copying BTrDB streams (or parts of streams) from one collection to another, potentially on a different server.

The utility will copy streams in parallel, but you shouldn't do more than about 15 at once.

To start, create a config file that looks like this:

```yaml
fromserver: my.source.server:4410
toserver: my.dest.server:4410
starttime: 2006-01-02T15:04:05+07:00
endtime: 2007-01-02T15:04:05+07:00
abortifexists: true
streams:
  - srccollection: my/collection
    dstcollection: my/dest/collection
    tags:
      name: L1MAG
  - srccollection: my/collection
    dstcollection: my/dest/collection
    tags:
      name: L2MAG
```

At present the (potentially partial) tags are used to identify the stream to copy, but the tags and annotations in the created streams are copied verbatim from the source

Then run

```bash
btrdbcp myconfig.yml
```
