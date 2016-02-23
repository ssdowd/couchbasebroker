# couchbasebroker
Cloud Foundry Service Broker for Couchbase

## Running

To run the broker, there are several modes.  The simplest is to run it standalone (make sure you have a GOPATH defined and that GOPATH/bin is in your PATH):

```
go get github.com/ssdowd/couchbasebroker
couchbasebroker --help
couchbasebroker \
  -config ${GOPATH}/src/github.com/ssdowd/couchbasebroker/assets/config.json \
  -copts ${GOPATH}/src/github.com/ssdowd/couchbasebroker/assets/boshconfig.json \
  -service Bosh
```  

A better idea is to create your own config (outside of the GOPATH) and copy/edit the config files from the GOPATH.

Things you need:

* data - directory to hold the data files (bindings and instances) and the bosh deployment files
* config/
* assets/


Another mode of operation is to deploy as a Cloud Foundry application