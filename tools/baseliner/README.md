# Baseliner

This is a quick tool for determining how many synchrophasor-like loads a BTrDB server can handle.

## Overview

Some parameters have to be modified in the code, but generally all testing can be done by just
using the interactive prompt. As with most BTrDB tools you set the server through an environment variable:

```
export BTRDB_ENDPOINTS=127.0.0.1:4410
```

When it starts up the program creates 10 workers, each with its own connection to BTrDB. You can
then type in the number of total streams you would like the program to emulate and it will increase or
decrease the number of emulated streams.

Every 15 seconds the program prints out a line like this:

```
total_inserts=1272 avg_time=13.946ms [+inserts=750 +avg_time=13.831ms p95=21.226ms]
```

This shows, in order:
- the total number of insert operations that have been performed since the program start. Each insert operation is 120 points.
- the average insert latency since program start
- the number of insert operations in the past 15 seconds
- the average insert latency over the past 15 seconds
- the 95th percentile insert latency over the past 15 seconds

There are some artificially inserted delays between stream creations to ensure the server doesn't see
synchronized load. If these need to be tweaked you will have to modify the code.

## Viewing the data

When it first starts up the tool will print a line like:

```
SET CODE IS a44f66
```

If you go into the plotter, all the data for this run will be under a collection called
`sim/<setcode>/`
