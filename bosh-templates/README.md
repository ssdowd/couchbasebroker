# This holds test yml files for assembling a dynamic couchbase deployment.

Files will be assembled using spruce merge.  It takes the first file as the baseline, then applies each subsequent file over it.  Last match wins.  Note the heredocs for director_uuid, deployment name and couchbase instance count (the newline and space are required).

Example:

```
spruce merge \
    --prune couchbase \
    base-cb-deploy.yml \
    network-bosh-lite.yml \
    couchbase-job-defaults.yml \
    stub.yml \
    <(echo "name: couchbase-d") \
    <(echo "director_uuid: `bosh status --uuid`") \
    <(echo "couchbase:\n instances: 3") \
  > deploythis-d.yml
```

Files are:

