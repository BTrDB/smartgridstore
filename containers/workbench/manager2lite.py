#!/usr/bin/python
from configobj import ConfigObj
import json
import os
import pymongo
import sys
import uuid

# ALL POSSIBLE STREAMS THAT A UPMU MAY HAVE (not all uPMUs may have all of these)
UPMU_STREAMS = {"L1MAG", "L1ANG", "L2MAG", "L2ANG", "L3MAG", "L3ANG","C1MAG", "C1ANG", "C2MAG", "C2ANG", "C3MAG", "C3ANG", "LSTATE", "FUND_W", "FUND_VAR", "FUND_VA", "FUND_DPF", "FREQ_L1_1S", "FREQ_L1_C37"}

use_backup = True
must_deploy = set()
if len(sys.argv) == 2:
    if sys.argv[1] == "--update-all":
        use_backup = False
    else:
        use_backup = True

for upmu in sys.argv[1:]:
    must_deploy.add(upmu)

def mergenesteddicts(base, overrides):
    """ Merges OVERRIDES into BASE, overriding properties where necessary. If
    additional dictionaries are contained as values, they are recursively
    merged. """
    for key in overrides:
        if key in base and isinstance(base[key], dict) and isinstance(overrides[key], dict):
                mergenesteddicts(base[key], overrides[key])
        else:
            base[key] = overrides[key]

def deepcopy(dictionary):
    newdict = {}
    for key in dictionary:
        if isinstance(dictionary[key], dict):
            newdict[key] = deepcopy(dictionary[key])
        else:
            newdict[key] = dictionary[key]
    return newdict

config = ConfigObj('/etc/sync/upmuconfig.ini')
if use_backup:
    oldconfig = ConfigObj('/etc/sync/backupconfig.ini')
else:
    oldconfig = {}
    
curr_upmus = set(config.keys())
old_upmus = set(oldconfig.keys())

client = pymongo.MongoClient(os.getenv("MONGO_ADDR","mongo.local"))
metadata = client.qdf.metadata

# Account for possible removal of uPMUs
for upmu in old_upmus - curr_upmus:
    print "Removing metadata for uPMU {0}".format(upmu)
    stillhasmetadata = False
    try:
        for stream in oldconfig[upmu]:
            if stream in UPMU_STREAMS:
                metadata.remove({"uuid": oldconfig[upmu][stream]['uuid']})
    except BaseException as be:
        print "ERROR: could not remove metadata for uPMU {0}: {1}".format(upmu, be)
        stillhasmetadata = True
    if stillhasmetadata:
        config['?' + trueupmu] = oldconfig[upmu]

for upmu in curr_upmus:
    deployed = True
    updatedmetadata = True
    print "Processing uPMU {0}".format(upmu)
    if upmu in oldconfig:
        old_metadata = oldconfig[upmu]
    elif ('?' + upmu) in oldconfig:
        old_metadata = oldconfig['?' + upmu]
    else:
        old_metadata = {}
    if old_metadata != config[upmu] or (upmu in must_deploy or ("%alias" in config[upmu] and config[upmu]["%alias"] in must_deploy)):
        try:   
            print "Updating metadata for uPMU {0}".format(upmu)
            # We have to update the database in this case
            collective_metadata = config[upmu].copy()
            streams = set()
            for stream in collective_metadata:
                if stream in UPMU_STREAMS:
                    streams.add(stream)
            for stream in streams:
                del collective_metadata[stream]
            keys = collective_metadata.keys()
            for key in keys:
                if len(key) >= 1 and key[0] == '%':
                    del collective_metadata[key]
            for stream in config[upmu]:
                if stream in UPMU_STREAMS:
                    newdoc = deepcopy(collective_metadata)
                    mergenesteddicts(newdoc, config[upmu][stream])
                    metadata.update({"uuid": config[upmu][stream]['uuid']}, newdoc, upsert = True)
        except BaseException as be:
            print "ERROR: could not update metadata on uPMU {0}: {1}".format(upmu, be)
            updatedmetadata = False
 
    if not deployed and not updatedmetadata:
        if upmu in oldconfig:
            config['?' + upmu] = oldconfig[upmu]
            config['?' + upmu]["%mustupdate"] = "true"
        else:
            config['?' + upmu] = {"%mustupdate": "true"}
        del config[upmu]
    elif not updatedmetadata:
        if upmu in oldconfig:
            config[upmu] = oldconfig[upmu]
            config[upmu]["%mustupdate"] = "true"
        else:
            config[upmu] = {"%mustupdate": "true"}
    elif not deployed:
        config['?' + upmu] = config[upmu]
        del config[upmu]

config.filename = '/etc/sync/backupconfig.ini'
config.write()
