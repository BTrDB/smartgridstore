Account Management CLI Tool for Mr. Plotter
===========================================

This CLI tool is for managing accounts for Mr. Plotter (the **M**ulti-**R**esolution **Plotter**). Accounts are stored in an etcd database that is accessed by the plotter backend.

Environment Variables
---------------------

Both of the following environment variables should be optionally set:
* ETCD_ENDPOINT - Should be set to the `host:port` of the etcd endpoint (if not set, uses `localhost:2379`)
* ETCD_KEY_PREFIX - Optionally allows the user to add a configuration-specific prefix to each key, allowing for multiple Mr. Plotter configurations

Using the CLI Tool
------------------
Compile the tool using `go get`. Then run the program. A list of commands can be accessed within the tool:
```
$ ./mr-plotter-accounts
Mr. Plotter Accounts> help
Type one of the following commands and press <Enter> or <Return> to execute it:
setpassword lstags lsusers close rmtags ls exit adduser rmuser rmusers addtags
```

Compatibility
-------------
This is fully compatible with the previous python-based tool; all commands and their old syntax will work with this one. However, some additional features have been added in this version.
