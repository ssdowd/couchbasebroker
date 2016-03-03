# Some example endpoint tests

username=$1
password=$2
port=7326

###---->> Get the catalog
curl -u ${username}:${password} -X GET http://localhost:${port}/v2/catalog

###---->> Create 2 Couchbase instances (111 and 222, requires a real working bosh or docker behind the broker)
# curl -u ${username}:${password} -X PUT http://localhost:${port}/v2/service_instances/instance_guid-111 -d '{
#   "service_id":"service-guid-111",
#   "plan_id":"b4c881e6-92ff-11e5-8436-60f81dc0df0a",
#   "organization_guid": "org-guid",
#   "space_guid":"space-guid",
#   "parameters": {"ami_id":"ami-ecb68a84"}
# }' -H "Content-Type: application/json"
#
# curl -u ${username}:${password} -X PUT http://localhost:${port}/v2/service_instances/instance_guid-222 -d '{
#   "service_id":"service-guid-222",
#   "plan_id":"cfe06e26-92ff-11e5-aaff-60f81dc0df0a",
#   "organization_guid": "org-guid",
#   "space_guid":"space-guid",
#   "parameters": {}
# }' -H "Content-Type: application/json"
#

###---->> check on the status of those 2 service instances... may take a while...
# curl -u ${username}:${password} -X GET http://localhost:${port}/v2/service_instances/instance_guid-111
# curl -u ${username}:${password} -X GET http://localhost:${port}/v2/service_instances/instance_guid-222
#

###---->> Try to bind and app to each instance
# curl -u ${username}:${password} -X PUT http://localhost:${port}/v2/service_instances/instance_guid-111/service_bindings/binding_guid-111 -d '{
#   "plan_id":        "b4c881e6-92ff-11e5-8436-60f81dc0df0a",
#   "service_id":     "service-guid-111",
#   "app_guid":       "app-guid"
# }' -H "Content-Type: application/json"
#
# curl -u ${username}:${password} -X PUT http://localhost:${port}/v2/service_instances/instance_guid-222/service_bindings/binding_guid-222 -d '{
#   "plan_id":        "cfe06e26-92ff-11e5-aaff-60f81dc0df0a",
#   "service_id":     "service-guid-222",
#   "app_guid":       "app-guid"
# }' -H "Content-Type: application/json"
#

###---->> Delete the service binding and service instance 111...
# curl -u ${username}:${password} -X DELETE http://localhost:${port}/v2/service_instances/instance_guid-111/service_bindings/binding_guid-111
# curl -u ${username}:${password} -X DELETE http://localhost:${port}/v2/service_instances/instance_guid-111

###---->> Delete the service binding and service instance 222...
# curl -u ${username}:${password} -X DELETE http://localhost:${port}/v2/service_instances/instance_guid-222/service_bindings/binding_guid-222
# curl -u ${username}:${password} -X DELETE http://localhost:${port}/v2/service_instances/instance_guid-222
