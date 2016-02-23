package client

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	// uuid "code.google.com/p/go-uuid/uuid"
	uuid "github.com/pborman/uuid"

	model "github.com/ssdowd/couchbasebroker/model"
	utils "github.com/ssdowd/couchbasebroker/utils"
	dockerclient "github.com/fsouza/go-dockerclient"
)

type dockerProps struct {
	hostname       string
	domain         string
	startCpus      int
	maxMemory      int
	dataCenterName string
	dockerImage    string
}

type DockerClient struct {
	dProps     dockerProps
	cbDefaults cbDefaultSettings
	catalog    *model.Catalog
}

func NewDockerClient() *DockerClient {
	utils.Logger.Println("NewDockerClient ready!")

	defaultProps := defaultDockerProperties()
	return &DockerClient{
		dProps: defaultProps,
	}
}

// state == pending, running, succeeded, failed
func (c *DockerClient) GetInstanceState(instanceId string) (string, error) {
	utils.Logger.Printf("client.docker.GetInstanceState: catalog: %v\n", *c.catalog)
	utils.Logger.Printf("client.docker.GetInstanceState: %v\n", instanceId)
	dclient, err := c.createDockerClient()
	if err != nil {
		utils.Logger.Printf("client.docker.GetInstanceState: error creating docker client: %v\n", err)
		return "failed", err
	}

	container, err := dclient.InspectContainer(instanceId)
	if err != nil {
		utils.Logger.Printf("client.docker.GetInstanceState: error on InspectContainer: %v\n", err)
		return "failed", err
	}

	if container.State.Running {
		ipaddr := container.NetworkSettings.IPAddress
		cbProps := cbDefaultProps()

		// now test the Couchbase instance at that address...
		client := &http.Client{}
		cburl := fmt.Sprintf("http://%s:%d", ipaddr, 8091)

		// if the default admin password does not work, we must have provisioned it...
		request, err := http.NewRequest("GET", fmt.Sprintf("%s/pools/default", cburl), nil)
		request.SetBasicAuth(cbProps.adminUser, cbProps.adminPass)
		response, err := client.Do(request)
		if err != nil {
			utils.Logger.Printf("client.docker.GetCredentials: error in http POST %v\n", err)
			// this is tricky - a connection refused can mean it's dead ot not warmed up yet.
			// but we don't want to wait here.
			return "pending", nil
		}
		switch response.StatusCode {
		case http.StatusUnauthorized:
			return "running", nil
		case http.StatusOK:
			return "pending", nil
		}

		return "pending", nil
	}

	return "pending", nil
}

func (c *DockerClient) IsValidPlan(planName string) bool {
	if c.catalog == nil {
		utils.Logger.Printf("client.docker.IsValidPlan: Cannot find the catalog")
		return false
	}

	// TODO: loop through services in the catalog, and look for a matching plan name
	for _, s := range c.catalog.Services {
		for _, p := range s.Plans {
			if p.Name == planName {
				return true
			}
		}
	}
	return false
}

// Equivalent of: docker run -d --name=cb-test couchbase
func (c *DockerClient) CreateInstance(parameters interface{}) (string, error) {
	// for now we ignore any parameters...

	// get a docker client
	dclient, err := c.createDockerClient()
	if err != nil {
		utils.Logger.Printf("client.docker.CreateInstance: error creating Docker client: %v\n", err)
		return "", err
	}

	utils.Logger.Printf("client.docker.CreateInstance: catalog: %v\n", *c.catalog)
	utils.Logger.Printf("client.docker.CreateInstance...Client: %v\n", dclient)
	copts := dockerclient.CreateContainerOptions{
		Config: &dockerclient.Config{
			Image: "couchbase",
		},
		HostConfig: &dockerclient.HostConfig{},
	}

	// Create a container, start ip, then inspect it to dump the IP...
	// the last part can happen when service bindings are requested...
	container, err := dclient.CreateContainer(copts)
	if err != nil {
		utils.Logger.Printf("client.docker..CreateInstance: error on CreateContainer: %v\n", err)
		return "", err
	}
	err = dclient.StartContainer(container.ID, copts.HostConfig)
	if err != nil {
		utils.Logger.Printf("client.docker.CreateInstance: error on StartContainer: %v\n", err)
		return "", err
	}
	container, err = dclient.InspectContainer(container.ID)
	if err != nil {
		utils.Logger.Printf("client.docker.CreateInstance: error on InspectContainer: %v\n", err)
		return "", err
	}
	utils.Logger.Printf("client.docker.CreateInstance OK %v\n\tIP: %v\n", container.ID, container.NetworkSettings.IPAddress)

	// finally, return the container ID for tracking
	return container.ID, nil
}

func (c *DockerClient) DeleteInstance(instanceId string) error {
	utils.Logger.Printf("client.docker.DeleteInstance: %v\n", instanceId)
	// get a docker client
	dclient, err := c.createDockerClient()
	if err != nil {
		utils.Logger.Printf("client.docker.DeleteInstance: error creating Docker client: %v\n", err)
		return err
	}

	container, err := dclient.InspectContainer(instanceId)
	if err != nil {
		utils.Logger.Printf("client.docker.DeleteInstance: error on InspectContainer: %v\n", err)
		return err
	}

	if !container.State.Running {
		return errors.New(fmt.Sprintf("client.docker.DeleteInstance: %v was not running", instanceId))
	}

	err = dclient.StopContainer(instanceId, 10)
	if err != nil {
		utils.Logger.Printf("client.docker.DeleteInstance: timeout on StopContainer %v\n", instanceId)
		return err
	}

	err = dclient.RemoveContainer(dockerclient.RemoveContainerOptions{
		ID:    instanceId,
		Force: true,
	})
	if err != nil {
		utils.Logger.Printf("client.docker.DeleteInstance: timeout on StopContainer %v\n", instanceId)
		return err
	}

	return nil
}

// This will configure the Couchbase instance with credentials and a bucket and other settings
func (c *DockerClient) GetCredentials(instanceId string) (*model.Credential, error) {
	utils.Logger.Printf("client.docker.GetCredentials: %v\n", instanceId)

	// get a docker client
	dclient, err := c.createDockerClient()
	if err != nil {
		utils.Logger.Printf("client.docker.GetCredentials: error creating Docker client: %v\n", err)
		return nil, err
	}

	container, err := dclient.InspectContainer(instanceId)
	if err != nil {
		utils.Logger.Printf("client.docker.GetCredentials: error on InspectContainer: %v\n", err)
		return nil, err
	}

	if !container.State.Running {
		return nil, errors.New(fmt.Sprintf("client.docker.GetCredentials: %v was not running", instanceId))
	}
	ipaddr := container.NetworkSettings.IPAddress
	cbProps := cbDefaultProps()

	// now configure the Couchbase instance at that address...
	client := &http.Client{}
	cburl := fmt.Sprintf("http://%s:%d", ipaddr, 8091)

	// ${CURL} -u Administrator:password -X POST http://${IP}:8091/pools/default -d memoryQuota=${MEMORYQUOTA}
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/pools/default", cburl), strings.NewReader(fmt.Sprintf("memoryQuota=%d", cbProps.ramQuota)))
	request.SetBasicAuth("Administrator", "password")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		utils.Logger.Printf("client.docker.GetCredentials: error in http POST %v\n", err)
		return nil, err
	} else {
		switch response.StatusCode {
		case http.StatusBadRequest, http.StatusUnauthorized:
			return nil, errors.New(fmt.Sprintf("Bad response from Couchbase(1): %v", response.StatusCode))
		}
	}

	// ${CURL} -u id:pw -X POST http://${IP}:8091/pools/default -d indexMemoryQuota=${INDEXQUOTA}
	request, err = http.NewRequest("POST", fmt.Sprintf("%s/pools/default", cburl), strings.NewReader(fmt.Sprintf("indexMemoryQuota=%d", cbProps.indexRAMQuota)))
	request.SetBasicAuth("Administrator", "password")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	response, err = client.Do(request)
	if err != nil {
		return nil, err
	} else {
		switch response.StatusCode {
		case http.StatusBadRequest, http.StatusUnauthorized:
			return nil, errors.New(fmt.Sprintf("Bad response from Couchbase(2): %v", response.StatusCode))
		}
	}

	// ${CURL} -u Administrator:password -X POST http://${IP}:8091/node/controller/setupServices -d services=${SERVICES}
	request, err = http.NewRequest("POST", fmt.Sprintf("%s/node/controller/setupServices", cburl), strings.NewReader("services=kv%2Cindex%2Cn1ql"))
	request.SetBasicAuth("Administrator", "password")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client = &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		return nil, err
	} else {
		switch response.StatusCode {
		case http.StatusBadRequest, http.StatusUnauthorized:
			return nil, errors.New(fmt.Sprintf("Bad response from Couchbase(3): %v", response.StatusCode))
		}
	}

	// override the default ID/password
	// ${CURL} -o /dev/null -u Administrator:password -X POST http://${IP}:8091/settings/web -d password=${PASSWORD} -d username=${USERNAME} -d port=8091
	userID := strings.Replace(uuid.NewRandom().String(), "-", "", -1)
	passwd := strings.Replace(uuid.NewRandom().String(), "-", "", -1)
	saslpasswd := strings.Replace(uuid.NewRandom().String(), "-", "", -1)
	bucketName := "cfdefault"
	credentials := model.Credential{
		URI:          cburl,
		UserName:     userID,
		Password:     passwd,
		SASLPassword: saslpasswd,
		BucketName:   bucketName,
	}
	utils.Logger.Printf("client.docker.GetCredentials: %v\n", credentials)
	request, err = http.NewRequest("POST",
		fmt.Sprintf("%s/settings/web", cburl),
		strings.NewReader(fmt.Sprintf("username=%s&password=%s&port=%d", credentials.UserName, credentials.Password, cbProps.port)))
	request.SetBasicAuth("Administrator", "password")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	response, err = client.Do(request)
	if err != nil {
		return nil, err
	} else {
		switch response.StatusCode {
		case http.StatusBadRequest, http.StatusUnauthorized:
			return nil, errors.New(fmt.Sprintf("Bad response from Couchbase(4): %v", response.StatusCode))
		}
	}

	// ${CURL} -u ${USERNAME}:${PASSWORD} -X POST http://${IP}:8091/pools/default/buckets \
	//   -d name=${BUCKET} -d bucketType=couchbase -d ramQuotaMB=${BUCKETRAM} -d proxyPort=9999 \
	//   -d authType=sasl -d saslPassword=${SASLPASSWORD}
	request, err = http.NewRequest("POST",
		fmt.Sprintf("%s/pools/default/buckets", cburl),
		strings.NewReader(fmt.Sprintf("name=%s&bucketType=couchbase&ramQuotaMB=%d&authType=sasl&saslPassword=%s", bucketName, 768, saslpasswd)))
	request.SetBasicAuth(userID, passwd)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	response, err = client.Do(request)
	if err != nil {
		return nil, err
	} else {
		switch response.StatusCode {
		case http.StatusBadRequest, http.StatusUnauthorized:
			return nil, errors.New(fmt.Sprintf("Bad response from Couchbase(5): %v", response.StatusCode))
		}
	}

	return &credentials, nil
}

func (c *DockerClient) RemoveCredentials(instanceId string, bindingId string) error {
	// we don't really remove credentials, since all instances will share the same ID/password
	// if we remove it, we'd need to reconfigure couchbase
	return nil
}

func (c *DockerClient) SetCatalog(catalog *model.Catalog) error {
	c.catalog = catalog
	return nil
}
func (c *DockerClient) GetCatalog() *model.Catalog {
	return c.catalog
}

// stubs
func (c *DockerClient) InjectKeyPair(instanceId string) (string, string, string, error) {
	return "", "", "", errors.New("InjectKeyPair not implemented for Docker")
}
func (c *DockerClient) RevokeKeyPair(instanceId string, privateKeyName string) error {
	return errors.New("RevokeKeyPair not implemented for Docker")
}

// Private methods

func (c *DockerClient) createDockerClient() (*dockerclient.Client, error) {
	endpoint := os.Getenv("DOCKER_HOST")
	if endpoint == "" {
		return nil, errors.New("You must set environment variable DOCKER_HOST for Docker cloud")
	}

	path := os.Getenv("DOCKER_CERT_PATH")
	if path == "" {
		return nil, errors.New("You must set environment variable DOCKER_CERT_PATH for Docker cloud")
	}
	ca := fmt.Sprintf("%s/ca.pem", path)
	cert := fmt.Sprintf("%s/cert.pem", path)
	key := fmt.Sprintf("%s/key.pem", path)

	return dockerclient.NewTLSClient(endpoint, cert, key, ca)
}

func (c *DockerClient) configure(ipAddress string) error {
	return nil
}

func defaultDockerProperties() dockerProps {
	return dockerProps{
		hostname:       "go-service-broker",
		domain:         "docker.com",
		startCpus:      1,
		maxMemory:      1024,
		dataCenterName: "ams01",
		dockerImage:    "couchbase:latest",
	}
}
