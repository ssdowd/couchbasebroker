# couchbasebroker
Cloud Foundry Service Broker for Couchbase

## Dependencies

This module depends on having the Couchbase release deployed in the target Bosh director instance defined in boshconfig.json.

## Running

To run the broker, there are several modes.  The simplest is to run it standalone (make sure you have a GOPATH defined and that GOPATH/bin is in your PATH):

```
go get github.com/ssdowd/couchbasebroker
```

Then you will need to set up some configuration for your broker instance in a directory where you will run the broker.  In that directory, you will need the following (all can be copied from $GOPATH/src/github.com/ssdowd/couchbasebroker):

* assets/config.json - basic configuration for the server
* assets/boshconfig.json - settings to talk to a bosh director (URL, ID, password)
* bosh-templates/ - directory containing the bosh templates from the Go source (copy all of $GOPATH/src/github.com/ssdowd/couchbasebroker/bosh-templates)
* data/ - directory to hold the data files (bindings and instances) and the bosh deployment files
* data/catalog.bosh-lite.json - a Cloud Foundry catalog file describing service offerings (not implemented for selection of options)


Now you can run the broker in standalone mode:

```
couchbasebroker --help
couchbasebroker
```  



Another mode of operation is to deploy as a Cloud Foundry application.  You can do this with a cf push.

Finally, you can wrap this package in a Bosh release, deploy it to a Bosh instance and then point the CF cloud controller at the service broker that is now running in Bosh.  The benefit of this model is that bosh will manage the service broker and ensure it is always running.
