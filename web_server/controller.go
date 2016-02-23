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
	DEFAULT_POLLING_INTERVAL_SECONDS = 10
)

type Controller struct {
	cloudName   string
	cloudClient client.Client

	instanceMap map[string]*model.ServiceInstance
	bindingMap  map[string]*model.ServiceBinding
}

func CreateController(cloudName string, cloudOptionsFile string, instanceMap map[string]*model.ServiceInstance, bindingMap map[string]*model.ServiceBinding) (*Controller, error) {
	cloudClient, err := createCloudClient(cloudName, cloudOptionsFile)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("controller.CreateController: Could not create cloud: %s client, message: %s", cloudName, err.Error()))
	}

	controller := &Controller{
		cloudName:   cloudName,
		cloudClient: cloudClient,

		instanceMap: instanceMap,
		bindingMap:  bindingMap,
	}

	err = controller.loadCatalog()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("controller.CreateController: Could not load catalog for cloud %s client, message: %s", cloudName, err.Error()))
	}
	return controller, nil
}

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

func (c *Controller) CreateServiceInstance(w http.ResponseWriter, r *http.Request) {
	instanceGuid := utils.ExtractVarsFromRequest(r, "service_instance_guid")
	utils.Logger.Printf("controller.CreateServiceInstance %v\n", instanceGuid)
	utils.Logger.Printf("controller.CreateServiceInstance REQUEST:\n%s\n\n", dumpRequest(r))

	var instance model.ServiceInstance

	err := utils.ProvisionDataFromRequest(r, &instance)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		utils.Logger.Printf("controller.CreateServiceInstance %v - error: %v\n", instanceGuid, err)
		return
	}
	utils.Logger.Printf("controller.CreateServiceInstance %v - data: %v\n", instanceGuid, instance)
	utils.Logger.Printf("controller.CreateServiceInstance %v - requested plan: %v\n", instanceGuid, instance.PlanId)

	if !c.cloudClient.IsValidPlan(instance.PlanId) {
		utils.Logger.Printf("controller.CreateServiceInstance %v - requested plan: %v not found\n", instanceGuid, instance.PlanId)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: need to pass the plan here as well??  instance.Parameters are user-passed parms
	instanceId, err := c.cloudClient.CreateInstance(instance.Parameters)
	if err != nil {
		utils.Logger.Printf("controller.CreateServiceInstance: cloudClient.CreateInstance returned: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Here we have the ID of the Docker container in instanceId
	utils.Logger.Printf("controller.CreateServiceInstance %v - instanceId: %v\n", instanceGuid, instanceId)

	instance.InternalId = instanceId
	instance.Id = utils.ExtractVarsFromRequest(r, "service_instance_guid")
	instance.LastOperation = &model.LastOperation{
		State:                    "in progress",
		Description:              "creating service instance...",
		AsyncPollIntervalSeconds: DEFAULT_POLLING_INTERVAL_SECONDS,
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

func (c *Controller) GetServiceInstance(w http.ResponseWriter, r *http.Request) {

	instanceId := utils.ExtractVarsFromRequest(r, "service_instance_guid")
	utils.Logger.Printf("controller.GetServiceInstance %v\n", instanceId)
	utils.Logger.Printf("controller.GetServiceInstance REQUEST:\n%s\n\n", dumpRequest(r))
	instance := c.instanceMap[instanceId]
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

func (c *Controller) RemoveServiceInstance(w http.ResponseWriter, r *http.Request) {
	utils.Logger.Println("controller.RemoveServiceInstance...")
	utils.Logger.Printf("controller.RemoveServiceInstance REQUEST:\n%s\n\n", dumpRequest(r))

	instanceId := utils.ExtractVarsFromRequest(r, "service_instance_guid")
	instance := c.instanceMap[instanceId]
	if instance == nil {
		w.WriteHeader(http.StatusGone)
		return
	}

	err := c.cloudClient.DeleteInstance(instance.InternalId)
	if err != nil {
		utils.Logger.Printf("controller.RemoveServiceInstance: %v error: %v\n", instanceId, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	delete(c.instanceMap, instanceId)
	utils.MarshalAndRecord(c.instanceMap, conf.DataPath, conf.ServiceInstancesFileName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = c.deleteAssociatedBindings(instanceId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	utils.Logger.Printf("controller.RemoveServiceInstance %s OK\n", instanceId)
	utils.WriteResponse(w, http.StatusOK, model.Message{Description: "deleted"})
}

// BIND
func (c *Controller) Bind(w http.ResponseWriter, r *http.Request) {

	bindingId := utils.ExtractVarsFromRequest(r, "service_binding_guid")
	instanceId := utils.ExtractVarsFromRequest(r, "service_instance_guid")

	utils.Logger.Printf("controller.Bind instanceId: %v, bindingId: %v\n", instanceId, bindingId)
	utils.Logger.Printf("controller.Bind REQUEST:\n%s\n\n", dumpRequest(r))

	instance := c.instanceMap[instanceId]
	if instance == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	utils.Logger.Printf("controller.Bind instance: %v\n", instance)

	binding := c.bindingMap[bindingId]
	response := model.CreateServiceBindingResponse{}
	if binding != nil {
		// then just return what was stored on the binding
		utils.Logger.Printf("controller.Bind: %v found in binding map\n", bindingId)
		response = model.CreateServiceBindingResponse{
			binding.Credential,
		}
	} else {
		if instance != nil {
			response = model.CreateServiceBindingResponse{
				instance.Credential,
			}
			// put into the binding table too...
			c.bindingMap[bindingId] = &model.ServiceBinding{
				Id:                bindingId,
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
			utils.Logger.Printf("controller.Bind: %v NOT found in binding map, checking instance %v\n", bindingId, instanceId)

			credential, err := c.cloudClient.GetCredentials(instance.InternalId)
			if err != nil {
				utils.Logger.Printf("controller.Bind: error in GetCredentials: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			response = model.CreateServiceBindingResponse{
				Credentials: *credential,
			}

			c.bindingMap[bindingId] = &model.ServiceBinding{
				Id:                bindingId,
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

func (c *Controller) UnBind(w http.ResponseWriter, r *http.Request) {
	utils.Logger.Printf("controller.UnBind REQUEST:\n%s\n\n", dumpRequest(r))

	bindingId := utils.ExtractVarsFromRequest(r, "service_binding_guid")
	instanceId := utils.ExtractVarsFromRequest(r, "service_instance_guid")
	utils.Logger.Printf("controller.UnBind bindingId: '%v', instanceId: '%v'\n", bindingId, instanceId)
	instance := c.instanceMap[instanceId]
	if instance == nil {
		utils.Logger.Printf("controller.UnBind instance not found\n")
		//		w.WriteHeader(http.StatusGone)
		utils.WriteResponse(w, http.StatusGone, model.Message{Description: "already gone"})
		return
	}

	err := c.cloudClient.RemoveCredentials(instance.InternalId, bindingId)
	if err != nil {
		utils.Logger.Printf("controller.UnBind error removing credentials\n")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	delete(c.bindingMap, bindingId)
	err = utils.MarshalAndRecord(c.bindingMap, conf.DataPath, conf.ServiceBindingsFileName)
	if err != nil {
		utils.Logger.Printf("controller.UnBind error deleting bindingId\n")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	utils.Logger.Printf("controller.UnBind OK\n")
	utils.WriteResponse(w, http.StatusOK, model.Message{Description: "deleted"})
}

// Private instance methods

func (c *Controller) deleteAssociatedBindings(instanceId string) error {
	for id, binding := range c.bindingMap {
		if binding.ServiceInstanceId == instanceId {
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

	return nil, errors.New(fmt.Sprintf("Invalid cloud name: %s", cloudName))
}

func (c *Controller) setupInstance(instanceGuid string, instanceId string) {
	time.Sleep(100 * time.Millisecond)
	instance := c.instanceMap[instanceGuid]
	if instance == nil {
		utils.Logger.Printf("controller.setupInstance: count not find instance: %v\n", instanceGuid)
		return
	}

	totalWait := 0
	interval := 1
	maxWait := 300
	var err error
	for totalWait < maxWait {
		credential, err := c.cloudClient.GetCredentials(instanceId)
		if err != nil {
			utils.Logger.Printf("controller.setupInstance: %v: %v\n", instanceId, err)
		} else {
			utils.Logger.Printf("controller.setupInstance: %v appears to be ready: %v\n", instanceId, credential)
			instance.DashboardUrl = credential.URI
			instance.Credential = *credential
			instance.LastOperation = &model.LastOperation{
				State:                    "running",
				Description:              "service instance ready...",
				AsyncPollIntervalSeconds: DEFAULT_POLLING_INTERVAL_SECONDS,
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
		AsyncPollIntervalSeconds: DEFAULT_POLLING_INTERVAL_SECONDS,
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
