package web_server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	client "github.com/ssdowd/couchbasebroker/client"
	model "github.com/ssdowd/couchbasebroker/model"
	utils "github.com/ssdowd/couchbasebroker/utils"
)

const (
	defaultPollingIntervalSeconds = 10
)

// A Controller holds the instance and binding maps for a given cloud and its client.
type Controller struct {
	cloudName   string
	cloudClient client.Client

	instanceMap map[string]*model.ServiceInstance
	bindingMap  map[string]*model.ServiceBinding
}

// CreateController returns a Controller for the given cloud with options, using the instance and binding maps provided.
func CreateController(cloudName string, cloudOptionsFile string, instanceMap map[string]*model.ServiceInstance, bindingMap map[string]*model.ServiceBinding) (*Controller, error) {
	cloudClient, err := createCloudClient(cloudName, cloudOptionsFile)
	if err != nil {
		return nil, fmt.Errorf("controller.CreateController: Could not create cloud: %s client, message: %s", cloudName, err.Error())
	}

	controller := &Controller{
		cloudName:   cloudName,
		cloudClient: cloudClient,

		instanceMap: instanceMap,
		bindingMap:  bindingMap,
	}

	err = controller.loadCatalog()
	if err != nil {
		return nil, fmt.Errorf("controller.CreateController: Could not load catalog for cloud %s client, message: %s", cloudName, err.Error())
	}
	return controller, nil
}

// Catalog implements the service broker REST endpoint for GET /v2/catalog.
func (c *Controller) Catalog(w http.ResponseWriter, r *http.Request) {
	utils.Logger.Printf("controller.Catalog REQUEST:\n%s\n\n", dumpRequest(r))

	err := c.loadCatalog()
	if err != nil {
		utils.Logger.Printf("controller.Catalog: error parsing config: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	utils.WriteResponse(w, http.StatusOK, c.cloudClient.GetCatalog())
}

func (c *Controller) loadCatalog() error {
	var catalog model.Catalog
	catalogFileName := "catalog.json"

	if c.cloudName == utils.AWS {
		catalogFileName = "catalog.AWS.json"
	} else if c.cloudName == utils.SOFTLAYER || c.cloudName == utils.SL {
		catalogFileName = "catalog.SoftLayer.json"
	} else if c.cloudName == utils.DOCKER {
		catalogFileName = "catalog.Docker.json"
	} else if c.cloudName == utils.BOSH {
		catalogFileName = "catalog.bosh-lite.json"
	}
	utils.Logger.Printf("loadCatalog: catalogFileName:'%s'\n", catalogFileName)

	err := utils.ReadAndUnmarshal(&catalog, conf.CatalogPath, catalogFileName)
	if err != nil {
		utils.Logger.Printf("controller.Catalog: error parsing %v/%v: %v\n", conf.CatalogPath, catalogFileName, err)
		return err
	}

	c.cloudClient.SetCatalog(&catalog)
	return nil
}

// CreateServiceInstance implements PUT /v2/service_instances/:id endpoint, creating a service instance from the given request.
func (c *Controller) CreateServiceInstance(w http.ResponseWriter, r *http.Request) {
	instanceGUID := utils.ExtractVarsFromRequest(r, "service_instance_guid")
	utils.Logger.Printf("controller.CreateServiceInstance %v\n", instanceGUID)
	utils.Logger.Printf("controller.CreateServiceInstance REQUEST:\n%s\n\n", dumpRequest(r))

	var instance model.ServiceInstance

	err := utils.ProvisionDataFromRequest(r, &instance)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		utils.Logger.Printf("controller.CreateServiceInstance %v - error: %v\n", instanceGUID, err)
		return
	}
	utils.Logger.Printf("controller.CreateServiceInstance %v - data: %v\n", instanceGUID, instance)
	utils.Logger.Printf("controller.CreateServiceInstance %v - requested plan: %v\n", instanceGUID, instance.PlanId)

	if !c.cloudClient.IsValidPlan(instance.PlanId) {
		utils.Logger.Printf("controller.CreateServiceInstance %v - requested plan: %v not found\n", instanceGUID, instance.PlanId)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: need to pass the plan here as well??  instance.Parameters are user-passed parms
	instanceID, err := c.cloudClient.CreateInstance(instance.Parameters)
	if err != nil {
		utils.Logger.Printf("controller.CreateServiceInstance: cloudClient.CreateInstance returned: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Here we have the ID of the Docker container in instanceID
	utils.Logger.Printf("controller.CreateServiceInstance %v - instanceID: %v\n", instanceGUID, instanceID)

	instance.InternalId = instanceID
	instance.Id = utils.ExtractVarsFromRequest(r, "service_instance_guid")
	instance.LastOperation = &model.LastOperation{
		State:                    "in progress",
		Description:              "creating service instance...",
		AsyncPollIntervalSeconds: defaultPollingIntervalSeconds,
	}

	c.instanceMap[instance.Id] = &instance
	err = utils.MarshalAndRecord(c.instanceMap, conf.DataPath, conf.ServiceInstancesFileName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		utils.Logger.Printf("controller.CreateServiceInstance: error saving instance map: %v\n", err)
		return
	}

	//=============================================================================================
	// Now set it up for client access - asynch
	// TODO: uncomment this...
	go c.setupInstance(instance.Id, instance.InternalId)
	//=============================================================================================

	response := model.CreateServiceInstanceResponse{
		DashboardUrl:  instance.DashboardUrl,
		LastOperation: instance.LastOperation,
	}
	utils.Logger.Printf("controller.CreateServiceInstance OK\n")
	utils.WriteResponse(w, http.StatusAccepted, response)
}

// GetServiceInstance implements the
// /v2/service_instances/{service_instance_guid} to allow the CF Cloud
// Controller to asynchronously poll for updates on a create.
func (c *Controller) GetServiceInstance(w http.ResponseWriter, r *http.Request) {

	instanceID := utils.ExtractVarsFromRequest(r, "service_instance_guid")
	utils.Logger.Printf("controller.GetServiceInstance %v\n", instanceID)
	utils.Logger.Printf("controller.GetServiceInstance REQUEST:\n%s\n\n", dumpRequest(r))
	instance := c.instanceMap[instanceID]
	if instance == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	state, err := c.cloudClient.GetInstanceState(instance.InternalId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	utils.Logger.Printf("controller.GetServiceInstance: state: %v\n", state)

	switch state {
	case "pending":
		instance.LastOperation.State = "in progress"
		instance.LastOperation.Description = "creating service instance..."
	case "running":
		instance.LastOperation.State = "in progress"
		instance.LastOperation.Description = "creating service instance..."
	case "succeeded":
		instance.LastOperation.State = "succeeded"
		instance.LastOperation.Description = "successfully created service instance"
		instance.LastOperation.DashboardUrl = instance.DashboardUrl
		instance.LastOperation.AsyncPollIntervalSeconds = 0
	case "failed":
		instance.LastOperation.State = "failed"
		instance.LastOperation.Description = "failed to create service instance"
	default:
		instance.LastOperation.State = "failed"
		instance.LastOperation.Description = "unknown state"
	}

	// response := model.CreateServiceInstanceResponse{
	//   DashboardUrl:  instance.DashboardUrl,
	//   LastOperation: instance.LastOperation,
	// }
	response := instance.LastOperation
	err = utils.MarshalAndRecord(c.instanceMap, conf.DataPath, conf.ServiceInstancesFileName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		utils.Logger.Printf("controller.CreateServiceInstance: error saving instance map: %v\n", err)
	}
	utils.WriteResponse(w, http.StatusOK, response)
}

// RemoveServiceInstance implements DELETE /v2/service_instances/:id endpoint, create a service instance from the given request.
func (c *Controller) RemoveServiceInstance(w http.ResponseWriter, r *http.Request) {
	utils.Logger.Println("controller.RemoveServiceInstance...")
	utils.Logger.Printf("controller.RemoveServiceInstance REQUEST:\n%s\n\n", dumpRequest(r))

	instanceID := utils.ExtractVarsFromRequest(r, "service_instance_guid")
	instance := c.instanceMap[instanceID]
	if instance == nil {
		w.WriteHeader(http.StatusGone)
		return
	}

	err := c.cloudClient.DeleteInstance(instance.InternalId)
	if err != nil {
		utils.Logger.Printf("controller.RemoveServiceInstance: %v error: %v\n", instanceID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	delete(c.instanceMap, instanceID)
	utils.MarshalAndRecord(c.instanceMap, conf.DataPath, conf.ServiceInstancesFileName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = c.deleteAssociatedBindings(instanceID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	utils.Logger.Printf("controller.RemoveServiceInstance %s OK\n", instanceID)
	utils.WriteResponse(w, http.StatusOK, model.Message{Description: "deleted"})
}

// Bind implements the service broker 2.7 PUT /v2/service_instances/:instance_id/service_bindings/:id.
func (c *Controller) Bind(w http.ResponseWriter, r *http.Request) {

	bindingID := utils.ExtractVarsFromRequest(r, "service_binding_guid")
	instanceID := utils.ExtractVarsFromRequest(r, "service_instance_guid")

	utils.Logger.Printf("controller.Bind instanceID: %v, bindingID: %v\n", instanceID, bindingID)
	utils.Logger.Printf("controller.Bind REQUEST:\n%s\n\n", dumpRequest(r))

	instance := c.instanceMap[instanceID]
	if instance == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	utils.Logger.Printf("controller.Bind instance: %v\n", instance)

	binding := c.bindingMap[bindingID]
	response := model.CreateServiceBindingResponse{}
	if binding != nil {
		// then just return what was stored on the binding
		utils.Logger.Printf("controller.Bind: %v found in binding map\n", bindingID)
		response = model.CreateServiceBindingResponse{
			Credentials: binding.Credential,
		}
	} else {
		if instance != nil {
			response = model.CreateServiceBindingResponse{
				Credentials: instance.Credential,
			}
			// put into the binding table too...
			c.bindingMap[bindingID] = &model.ServiceBinding{
				Id:                bindingID,
				ServiceId:         instance.ServiceId,
				ServicePlanId:     instance.PlanId,
				ServiceInstanceId: instance.Id,
				Credential:        instance.Credential,
				// Credential: model.Credential{
				//   URI:          instance.Credential.URI,
				//   UserName:     instance.Credential.UserName,
				//   Password:     instance.Credential.Password,
				//   SASLPassword: instance.Credential.SASLPassword,
				//   BucketName:   instance.Credential.BucketName,
				// },
			}
			err := utils.MarshalAndRecord(c.bindingMap, conf.DataPath, conf.ServiceBindingsFileName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

		} else {
			utils.Logger.Printf("controller.Bind: %v NOT found in binding map, checking instance %v\n", bindingID, instanceID)

			credential, err := c.cloudClient.GetCredentials(instance.InternalId)
			if err != nil {
				utils.Logger.Printf("controller.Bind: error in GetCredentials: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			response = model.CreateServiceBindingResponse{
				Credentials: *credential,
			}

			c.bindingMap[bindingID] = &model.ServiceBinding{
				Id:                bindingID,
				ServiceId:         instance.ServiceId,
				ServicePlanId:     instance.PlanId,
				ServiceInstanceId: instance.Id,
				Credential:        *credential,
			}
			err = utils.MarshalAndRecord(c.bindingMap, conf.DataPath, conf.ServiceBindingsFileName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

		}
	}

	utils.WriteResponse(w, http.StatusCreated, response)
}

// UnBind implements the service broker 2.7 DELETE /v2/service_instances/:instance_id/service_bindings/:id.
func (c *Controller) UnBind(w http.ResponseWriter, r *http.Request) {
	utils.Logger.Printf("controller.UnBind REQUEST:\n%s\n\n", dumpRequest(r))

	bindingID := utils.ExtractVarsFromRequest(r, "service_binding_guid")
	instanceID := utils.ExtractVarsFromRequest(r, "service_instance_guid")
	utils.Logger.Printf("controller.UnBind bindingID: '%v', instanceID: '%v'\n", bindingID, instanceID)
	instance := c.instanceMap[instanceID]
	if instance == nil {
		utils.Logger.Printf("controller.UnBind instance not found\n")
		//		w.WriteHeader(http.StatusGone)
		utils.WriteResponse(w, http.StatusGone, model.Message{Description: "already gone"})
		return
	}

	err := c.cloudClient.RemoveCredentials(instance.InternalId, bindingID)
	if err != nil {
		utils.Logger.Printf("controller.UnBind error removing credentials\n")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	delete(c.bindingMap, bindingID)
	err = utils.MarshalAndRecord(c.bindingMap, conf.DataPath, conf.ServiceBindingsFileName)
	if err != nil {
		utils.Logger.Printf("controller.UnBind error deleting bindingID\n")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	utils.Logger.Printf("controller.UnBind OK\n")
	utils.WriteResponse(w, http.StatusOK, model.Message{Description: "deleted"})
}

// Private instance methods

func (c *Controller) deleteAssociatedBindings(instanceID string) error {
	for id, binding := range c.bindingMap {
		if binding.ServiceInstanceId == instanceID {
			delete(c.bindingMap, id)
		}
	}

	return utils.MarshalAndRecord(c.bindingMap, conf.DataPath, conf.ServiceBindingsFileName)
}

// Private methods

func createCloudClient(cloudName string, cloudOptionsFile string) (client.Client, error) {
	switch cloudName {
	case utils.DOCKER:
		return client.NewDockerClient(), nil
	case utils.BOSH:
		return client.NewBoshClient(cloudOptionsFile), nil
	}

	return nil, fmt.Errorf("Invalid cloud name: %s", cloudName)
}

func (c *Controller) setupInstance(instanceGUID string, instanceID string) {
	time.Sleep(100 * time.Millisecond)
	instance := c.instanceMap[instanceGUID]
	if instance == nil {
		utils.Logger.Printf("controller.setupInstance: count not find instance: %v\n", instanceGUID)
		return
	}

	totalWait := 0
	interval := 1
	maxWait := 300
	var err error
	for totalWait < maxWait {
		credential, err := c.cloudClient.GetCredentials(instanceID)
		if err != nil {
			utils.Logger.Printf("controller.setupInstance: %v: %v\n", instanceID, err)
		} else {
			utils.Logger.Printf("controller.setupInstance: %v appears to be ready: %v\n", instanceID, credential)
			instance.DashboardUrl = credential.URI
			instance.Credential = *credential
			instance.LastOperation = &model.LastOperation{
				State:                    "running",
				Description:              "service instance ready...",
				AsyncPollIntervalSeconds: defaultPollingIntervalSeconds,
			}
			err = utils.MarshalAndRecord(c.instanceMap, conf.DataPath, conf.ServiceInstancesFileName)
			if err != nil {
				utils.Logger.Printf("controller.setupInstance: error saving instance map: %v\n", err)
			}
			return
		}
		time.Sleep(time.Duration(interval) * time.Second)
		// decaying interval...
		totalWait += interval
		if interval < 10 {
			interval = interval * 2
		}
	}

	if err == nil {
		err = errors.New("Unknown error")
	}
	instance.LastOperation = &model.LastOperation{
		State:                    "failed",
		Description:              fmt.Sprintf("failed to configure service instance: %v", err),
		AsyncPollIntervalSeconds: defaultPollingIntervalSeconds,
	}

	err = utils.MarshalAndRecord(c.instanceMap, conf.DataPath, conf.ServiceInstancesFileName)
	if err != nil {
		utils.Logger.Printf("controller.setupInstance: error saving instance map: %v\n", err)
		return
	}

}

func dumpRequest(request *http.Request) string {
	data, err := httputil.DumpRequest(request, true)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	return string(data)
}

func dumpResponse(response *http.Response) string {
	data, err := httputil.DumpResponse(response, true)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	return string(data)
}
