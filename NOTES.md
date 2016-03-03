NOTES
===

Notes for using this service broker.

main.go is the main class to invoke (run from this directory):

```
go run main.go
```

For the BOSH broker, use this:

```
ulimit -n 8192
# Ensure routes to 10.254.x.x are set up (through the bosh-lite IP)
# sudo route add -net 10.254.0.0/16 192.168.50.4
GOGOBOSH_TRACE=gogobosh.log go run main.go --service Bosh --copts assets/boshconfig.json
# OR
bin/build
GOGOBOSH_TRACE=gogobosh.log out/cb_service_broker --service Bosh --copts assets/boshconfig.json
```

Command line options

* --config path/to/config (default: assets/config.json)
* --service CLOUD (default: BOSH)

## Vendoring

I used glide for vendoring here.  Things to note: you have to do your development under $GOPATH/src/github.com/ssdowd/couchbasebroker.  When go gets that, it's a git clone (https), so it's under VCS.  (This is not obvious from reading Go docs.  _You may need to add an alternate remote to push back to github via ssh.  Only for the author and accomplices..._)

Second, once you have a copy you can use glide (```brew install glide```) to install the dependencies:

```
glide install
```

This is required because I chose not to commit the /vendor directory.

**Update** This has been converted to use godep, so dependencies are in Godeps/.  Set up your environment as follows:

```
go get github.com/tools/godep
godep restore
```
**Note: this is destructive to your $GOPATH.  Also, glide may still work, but godep seems to be the more common method.**

These pages are useful: 

* [https://github.com/Masterminds/glide]()
* [http://engineeredweb.com/blog/2015/go-1.5-vendor-handling/]()

## Lint, vet, test, ginkgo, etc.

Run **golint** as follows:

```
go get -u github.com/golang/lint/golint
golint ./...
```
Note: I did not fix lint warnings on packages from the original go_broker.

Run **vet** as follows:

```
go tool vet -all $(ls -d */ | grep -v Godeps)
```

Run **test** as follows:

```
go test
# OR
go test ./...
```

Run **ginkgo** as follows:

```
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega 
ginkgo
```

## TODO

* Tie the plans to what gets created.  The sizes in the plan should be passed in when creating an instance.
* Test this running in Cloud Foundry.
  * Issue: networking between Docker machine and CF

##Test the endpoints:

### Catalog

* GET the catalog:

```
curl http://localhost:7326/v2/catalog
```

### Service Instances
* GET a service instance:

```
curl -X GET http://localhost:7326/v2/service_instances/{service_instance_guid}
```

* Create (PUT) a new service instance:

```
curl -X PUT http://localhost:7326/v2/service_instances/123 -d '{
  "organization_guid": "org-123",
  "plan_id":           "plan-123",
  "service_id":        "service-123",
  "space_guid":        "space-123",
  "parameters":        {
    "parameter1": 1,
    "parameter2": "value"
 } }' -H "X-Broker-API-Version: 2.7" -H "Content-Type: application/json"
```

* DELETE a service instance:

```
curl -X DELETE http://localhost:7326/v2/service_instances/{service_instance_guid}
```

### Service Bindings

* Create (PUT) a new service binding:

```
curl -X PUT http://localhost:7326/v2/service_instances/{service_instance_guid}/service_bindings/{service_binding_guid} -d '{                                                                          1 ↵
  "plan_id":      "plan-123",
  "service_id":   "service-123",
  "app_guid":     "app-123",
  "parameters":   {
    "parameter1": 1,
    "parameter2": "value"
 } }' -H "X-Broker-API-Version: 2.7" -H "Content-Type: application/json"
```

* DELETE a service binding:

```
curl -X DELETE http://localhost:7326/v2/service_instances/{service_instance_guid}/service_bindings/{service_binding_guid}
```


