# Some example endpoint tests

username=someuser
password=somepass
port=7326

curl -X GET http://${username}:${password}@localhost:${port}/v2/catalog

curl -X PUT http://${username}:${password}@localhost:${port}/v2/service_instances/instance_guid-111 -d '{
  "service_id":"service-guid-111",
  "plan_id":"b4c881e6-92ff-11e5-8436-60f81dc0df0a",
  "organization_guid": "org-guid",
  "space_guid":"space-guid",
  "parameters": {"ami_id":"ami-ecb68a84"}
}' -H "Content-Type: application/json"

curl -X PUT http://${username}:${password}@localhost:${port}/v2/service_instances/instance_guid-222 -d '{
  "service_id":"service-guid-222",
  "plan_id":"cfe06e26-92ff-11e5-aaff-60f81dc0df0a",
  "organization_guid": "org-guid",
  "space_guid":"space-guid",
  "parameters": {}
}' -H "Content-Type: application/json"

curl -X GET http://${username}:${password}@localhost:${port}/v2/service_instances/instance_guid-111
curl -X GET http://${username}:${password}@localhost:${port}/v2/service_instances/instance_guid-222

curl -X PUT http://${username}:${password}@localhost:${port}/v2/service_instances/instance_guid-111/service_bindings/binding_guid-111 -d '{
  "plan_id":        "b4c881e6-92ff-11e5-8436-60f81dc0df0a",
  "service_id":     "service-guid-111",
  "app_guid":       "app-guid"
}' -H "Content-Type: application/json"

curl -X PUT http://${username}:${password}@localhost:${port}/v2/service_instances/instance_guid-222/service_bindings/binding_guid-222 -d '{
  "plan_id":        "cfe06e26-92ff-11e5-aaff-60f81dc0df0a",
  "service_id":     "service-guid-222",
  "app_guid":       "app-guid"
}' -H "Content-Type: application/json"

curl -X DELETE http://${username}:${password}@localhost:${port}/v2/service_instances/instance_guid-111/service_bindings/binding_guid-111
curl -X DELETE http://${username}:${password}@localhost:${port}/v2/service_instances/instance_guid-111

curl -X DELETE http://${username}:${password}@localhost:${port}/v2/service_instances/instance_guid-222/service_bindings/binding_guid-222
curl -X DELETE http://${username}:${password}@localhost:${port}/v2/service_instances/instance_guid-222
