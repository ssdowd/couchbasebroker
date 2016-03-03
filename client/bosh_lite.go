package client

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	// uuid "code.google.com/p/go-uuid/uuid"
	uuid "github.com/pborman/uuid"
	config "github.com/ssdowd/couchbasebroker/config"
	model "github.com/ssdowd/couchbasebroker/model"
	utils "github.com/ssdowd/couchbasebroker/utils"

	"github.com/ssdowd/gogobosh"
	"github.com/ssdowd/gogobosh/api"
	"github.com/ssdowd/gogobosh/net"
)

// A BoshClient manages aconnection to a BOSH director.
type BoshClient struct {
	dProps     *config.BoshConfig
	cbDefaults cbDefaultSettings
	catalog    *model.Catalog
	tasks      map[string]int
}

// spruce merge --prune Xname --prune couchbase base-cb-deploy.yml
//   network-bosh-lite.yml resources-bosh-lite.yml  couchbase-job-defaults.yml  stub.yml
//   <(echo "director_uuid: `bosh status --uuid`")
//   <(echo "couchbase:\n instances: 3") some-other-job-defaults.yml
var yamlList = []string{
	"base-cb-deploy.yml",
	"network-bosh-lite.yml",
	"resources-bosh-lite.yml",
	"couchbase-job-defaults.yml",
	"stub.yml",
}

// NewBoshClient creates and returns a BoshClient for use in working with a BOSH director.
func NewBoshClient(configFile string) *BoshClient {
	// utils.Logger.Printf("NewBoshClient %v\n", configFile)
	_, err := config.LoadBoshConfig(configFile)
	if err != nil {
		utils.Logger.Printf("NewBoshClient error loading Bosh config %v: %v\n", configFile, err)
	}

	defaultProps := config.GetBoshConfig()
	return &BoshClient{
		dProps: defaultProps,
		tasks:  make(map[string]int),
	}
}

// GetInstanceState returns a string indicating the state of the instance.
// state == pending, running, succeeded, failed
func (c *BoshClient) GetInstanceState(instanceID string) (string, error) {
	// utils.Logger.Printf("client.bosh.GetInstanceState: catalog: %v\n", *c.catalog)
	utils.Logger.Printf("client.bosh.GetInstanceState: instanceID: %v: task ID: %v\n", instanceID, c.tasks[instanceID])

	// we don't have a task for it, assume it is good...
	if c.tasks[instanceID] == 0 {
		return "succeeded", nil
	}

	boshclient, err := c.createBoshClient()
	if err != nil {
		utils.Logger.Printf("client.bosh.GetInstanceState: error creating bosh client: %v\n", err)
		return "failed", err
	}
	taskStatus, apiResponse := boshclient.GetTaskStatus(c.tasks[instanceID])
	if apiResponse.IsNotSuccessful() {
		utils.Logger.Printf("client.bosh.GetInstanceState... gogo.GetTaskStatus error: %v\n", apiResponse)
	}
	utils.Logger.Printf("client.bosh.GetInstanceState... taskStatus: %v\n", taskStatus)

	// map from gogobosh TaskStatus.State to the CF API states
	switch taskStatus.State {
	case "done":
		return "succeeded", nil
	case "processing":
		return "running", nil
	case "queued":
		return "pending", nil
	case "error":
		return "failed", nil
	default:
		return "failed", fmt.Errorf("Unknown bosh status: %v", taskStatus.State)
	}
}

// IsValidPlan checks the given planName to ensure it appears in the catalog.
func (c *BoshClient) IsValidPlan(planName string) bool {
	if c.catalog == nil {
		utils.Logger.Printf("client.bosh.IsValidPlan: Cannot find the catalog")
		return false
	}

	// TODO: loop through services in the catalog, and look for a matching plan name
	for _, s := range c.catalog.Services {
		for _, p := range s.Plans {
			if p.ID == planName {
				return true
			}
		}
	}
	return false
}

// CreateInstance is the qquivalent of: bosh run -d --name=cb-test couchbase.
func (c *BoshClient) CreateInstance(parameters interface{}) (string, error) {
	utils.Logger.Printf("client.bosh.CreateInstance parms: %v\n", parameters)
	// for now we ignore any parameters...

	// get a bosh client
	boshclient, err := c.createBoshClient()
	if err != nil {
		utils.Logger.Printf("client.bosh.CreateInstance: error creating Bosh client: %v\n", err)
		return "", err
	}
	info, apiResponse := boshclient.GetInfo()
	if apiResponse.IsNotSuccessful() {
		utils.Logger.Printf("client.bosh.CreateInstance: Could not fetch BOSH info %v\n", apiResponse)
		return "", errors.New("BOSH error")
	}

	deploymentName := "cb-" + strings.Replace(uuid.NewRandom().String(), "-", "", -1)[:10]
	// utils.Logger.Printf("client.bosh.CreateInstance: catalog: %v\n", *c.catalog)
	// utils.Logger.Printf("client.bosh.CreateInstance...Client: %v\n", boshclient)
	utils.Logger.Printf("client.bosh.CreateInstance...BOSH Deployment name: %v\n", deploymentName)
	utils.Logger.Printf("client.bosh.CreateInstance...BOSH Director UUID: %v\n", info.UUID)

	// did they put an instance count in the params? - it appears to be a float...
	instances := 1
	switch parameters.(type) {
	case map[string]interface{}:
		param := parameters.(map[string]interface{})
		if param["instances"] != nil {
			instances = int(param["instances"].(float64))
		}
	default:
		utils.Logger.Printf("client.bosh.CreateInstance... unmatched parameter type\n")
	}

	// Create a deployment.yml file by invoking spruce...
	args := []string{"merge"}
	templateDir := c.dProps.TemplateDir
	if !strings.HasPrefix(templateDir, string(os.PathSeparator)) {
		templateDir = utils.GetPath([]string{templateDir})
	}

	for _, val := range yamlList {
		args = append(args, templateDir+string(os.PathSeparator)+val)
	}
	// write variable portion to a tempfile (name, director UUID, instance count)
	f, err := ioutil.TempFile("", "bosh-deploy-tmp-")
	if err != nil {
		panic(err)
	}
	f.WriteString(fmt.Sprintf("name: %v\n", deploymentName))
	f.WriteString(fmt.Sprintf("director_uuid: %v\n", info.UUID))
	f.WriteString(fmt.Sprintf("couchbase:\n  instances: %v\n", instances))
	f.Close()
	args = append(args, f.Name())
	utils.Logger.Printf("client.bosh.CreateInstance: command args: %v\n", args)
	cmd := exec.Command("spruce", args...)
	utils.Logger.Printf("client.bosh.CreateInstance: command: %v\n", cmd)

	// make sure the deployment file directory exists
	err = os.MkdirAll(c.dProps.DataDir, 0750)
	if err != nil {
		panic(err)
	}

	// create the output (deployment yml) file, attach it to the command execution (shell redirect)
	fileName := c.dProps.DataDir + string(os.PathSeparator) + deploymentName + ".yml"
	utils.Logger.Printf("client.bosh.CreateInstance: deployment file: '%v'\n", fileName)
	outfile, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}
	defer outfile.Close()
	cmd.Stdout = outfile

	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	cmd.Wait()
	utils.Logger.Printf("client.bosh.CreateInstance: finished creating %v\n", fileName)

	//==================================================================================================
	// Now deploy that file using an HTTP POST
	datReader, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	req, _ := http.NewRequest("POST", c.dProps.DirectorURL+"/deployments", datReader)
	req.Header.Set("Content-Type", "text/yaml")
	req.SetBasicAuth(c.dProps.DirectorUser, c.dProps.DirectorPassword)
	utils.Logger.Printf("client.bosh.CreateInstance... request: \n%s\n\n", c.dumpRequest(req))
	// be promiscuous about SSL, don't follow redirects (we expect a task URL)
	client := &http.Client{
		Transport:     tr,
		CheckRedirect: noRedirect,
	}
	resp, err := client.Do(req)
	if err != nil {
		// TODO: check for something other than a redirect error
		// we need the func to return an error, otherwise we fail.
		utils.Logger.Printf("Ignoring 'error': %v\n", err)
	}
	utils.Logger.Printf("client.bosh.CreateInstance... response: \n%s\n\n", c.dumpResponse(resp))
	switch resp.StatusCode {
	case http.StatusFound:
		taskURL := resp.Header["Location"][0]
		utils.Logger.Printf("client.bosh.CreateInstance taskURL: '%v'\n", taskURL)
		chunks := strings.Split(taskURL, "/")
		taskID, err := strconv.Atoi(chunks[len(chunks)-1])
		if err != nil {
			panic(err)
		}
		c.tasks[deploymentName] = taskID
		// return the container ID for tracking
		// the monitoring will be done by GetCredentials, called by the controller
		utils.Logger.Printf("client.bosh.CreateInstance waitAndConfigure taskID: '%v'\n", taskID)
		c.waitAndConfigure(taskID)

		return deploymentName, nil
	default:
		// there is no body on this, but we'll read it anyway...
		body, _ := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		return "", fmt.Errorf("error POSTing deployment: %s: %v", resp.Status, body)
	}
}

func noRedirect(req *http.Request, via []*http.Request) error {
	return fmt.Errorf("Don't redirect to %v", req)
}

// DeleteInstance deletes the instance in Bosh with the associated instanceID.
func (c *BoshClient) DeleteInstance(instanceID string) error {
	utils.Logger.Printf("client.bosh.DeleteInstance: %v\n", instanceID)
	// get a bosh client
	boshclient, err := c.createBoshClient()
	if err != nil {
		utils.Logger.Printf("client.bosh.DeleteInstance: error creating Bosh client: %v\n", err)
		return err
	}
	utils.Logger.Printf("client.bosh.DeleteInstance...Client: %v\n", boshclient)
	fileName := c.dProps.DataDir + string(os.PathSeparator) + instanceID + ".yml"

	apiResponse := boshclient.DeleteDeployment(instanceID)
	if apiResponse.IsNotSuccessful() {
		utils.Logger.Printf("client.bosh.DeleteInstance: failed to delete deployment %v: %v", instanceID, apiResponse)
		return fmt.Errorf("failed to delete %v", instanceID)
	}
	err = os.Remove(fileName)
	if err != nil {
		utils.Logger.Printf(fmt.Sprintf("client.bosh.DeleteInstance: could not remove %v: %v\n", fileName, err))
	}
	return nil
}

// GetCredentials will configure the Couchbase instance with credentials and a bucket and other settings
func (c *BoshClient) GetCredentials(instanceID string) (*model.Credential, error) {
	// utils.Logger.Printf("client.bosh.GetCredentials: %v\n", instanceID)

	// get a bosh client
	boshclient, err := c.createBoshClient()
	if err != nil {
		utils.Logger.Printf("client.bosh.GetCredentials: error creating Bosh client: %v\n", err)
		return nil, err
	}
	taskStatus, apiResponse := boshclient.GetTaskStatus(c.tasks[instanceID])
	if apiResponse.IsNotSuccessful() {
		utils.Logger.Printf("client.bosh.GetCredentials... gogo.GetTaskStatus apiResponse: %v\n", apiResponse)
	}
	// utils.Logger.Printf("client.bosh.GetCredentials... taskStatus: %v\n", taskStatus)
	switch taskStatus.State {
	case "done": // get the IPs and configure them, return credentials
	case "success":
	case "queued":
		return nil, errors.New("task queued")
	case "processing":
		return nil, errors.New("task processing")
	case "in progress":
		return nil, errors.New("task in progress")
	case "error":
		return nil, errors.New("error")
	case "failed":
		return nil, errors.New("failed")
	default:
		return nil, errors.New("unknown task status: " + taskStatus.State)
	}

	// now configure the Couchbase instances at those addresses...
	boshclient, err = c.createBoshClient()
	if err != nil {
		utils.Logger.Printf("client.bosh.GetCredentials: error creating Bosh client: %v\n", err)
		return nil, err
	}
	vmStatuses, apiResponse := boshclient.FetchVMsStatus(instanceID)
	if apiResponse.IsNotSuccessful() {
		utils.Logger.Printf("client.bosh.GetCredentials... gogo.FetchVMsStatus: %v\n", apiResponse)
		return nil, fmt.Errorf("Could not invoke gogo.FetchVMsStatus: %v", apiResponse.Message)
	}
	var cred *model.Credential
	cred = nil
	userID := strings.Replace(uuid.NewRandom().String(), "-", "", -1)
	passwd := strings.Replace(uuid.NewRandom().String(), "-", "", -1)
	saslpasswd := strings.Replace(uuid.NewRandom().String(), "-", "", -1)
	var iplist = make([]string, len(vmStatuses))
	for i, vmStat := range vmStatuses {
		ip := vmStat.IPs[0]
		iplist[i] = ip
		if cred != nil {
			_, err = c.configureCouchbaseInstance(ip, userID, passwd, saslpasswd)
		} else {
			cred, err = c.configureCouchbaseInstance(ip, userID, passwd, saslpasswd)
		}
		if err != nil {
			return nil, err
		}
	}
	// setup cluster
	if len(iplist) > 1 {
		c.configureCouchbaseCluster(iplist, userID, passwd)
	}
	return cred, nil
}

// RemoveCredentials does not really remove credentials for Couchbase, since
// there may be other app instances bound to this service instance, or they may
// want to reuse that instance.
func (c *BoshClient) RemoveCredentials(instanceID string, bindingID string) error {
	// we don't really remove credentials, since all instances will share the same ID/password
	// if we remove it, we'd need to reconfigure couchbase
	return nil
}

// SetCatalog sets the catalog object for this broker.
func (c *BoshClient) SetCatalog(catalog *model.Catalog) error {
	c.catalog = catalog
	return nil
}

// GetCatalog returns the catalog object for this broker.
func (c *BoshClient) GetCatalog() *model.Catalog {
	return c.catalog
}

// InjectKeyPair is a stub to implement the client API.
func (c *BoshClient) InjectKeyPair(instanceID string) (string, string, string, error) {
	return "", "", "", errors.New("InjectKeyPair not implemented for Bosh")
}

// RevokeKeyPair is a stub to implement the client API.
func (c *BoshClient) RevokeKeyPair(instanceID string, privateKeyName string) error {
	return errors.New("RevokeKeyPair not implemented for Bosh")
}

// Private methods

func (c *BoshClient) createBoshClient() (api.BoshDirectorRepository, error) {
	director := gogobosh.NewDirector(c.dProps.DirectorURL, c.dProps.DirectorUser, c.dProps.DirectorPassword)
	return api.NewBoshDirectorRepository(&director, net.NewDirectorGateway()), nil
}

func (c *BoshClient) configure(ipAddress string) error {
	return nil
}

func (c *BoshClient) waitAndConfigure(taskID int) {
	utils.Logger.Printf("waitAndConfigure task: %v\n", taskID)
}

func (c *BoshClient) configureCouchbaseInstance(ipaddr, userID, passwd, saslpasswd string) (*model.Credential, error) {
	client := &http.Client{}
	cburl := fmt.Sprintf("http://%s:%d", ipaddr, 8091)
	cbProps := cbDefaultProps()

	// ${CURL} -u Administrator:password -X POST http://${IP}:8091/pools/default -d memoryQuota=${MEMORYQUOTA}
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/pools/default", cburl), strings.NewReader(fmt.Sprintf("memoryQuota=%d", cbProps.ramQuota)))
	request.SetBasicAuth("Administrator", "password")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		utils.Logger.Printf("client.bosh.configureCouchbaseInstance: error in http POST %v\n", err)
		return nil, err
	}
	switch response.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized:
		return nil, fmt.Errorf("Bad response from Couchbase(1): %v", response.StatusCode)
	}
	defer response.Body.Close()

	// ${CURL} -u id:pw -X POST http://${IP}:8091/pools/default -d indexMemoryQuota=${INDEXQUOTA}
	request, err = http.NewRequest("POST", fmt.Sprintf("%s/pools/default", cburl), strings.NewReader(fmt.Sprintf("indexMemoryQuota=%d", cbProps.indexRAMQuota)))
	request.SetBasicAuth("Administrator", "password")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	response, err = client.Do(request)
	if err != nil {
		return nil, err
	}
	switch response.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized:
		return nil, fmt.Errorf("Bad response from Couchbase(2): %v", response.StatusCode)
	}
	defer response.Body.Close()

	// ${CURL} -u Administrator:password -X POST http://${IP}:8091/node/controller/setupServices -d services=${SERVICES}
	request, err = http.NewRequest("POST", fmt.Sprintf("%s/node/controller/setupServices", cburl), strings.NewReader("services=kv%2Cindex%2Cn1ql"))
	request.SetBasicAuth("Administrator", "password")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client = &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		return nil, err
	}
	switch response.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized:
		return nil, fmt.Errorf("Bad response from Couchbase(3): %v", response.StatusCode)
	}
	defer response.Body.Close()

	// override the default ID/password
	// ${CURL} -o /dev/null -u Administrator:password -X POST http://${IP}:8091/settings/web -d password=${PASSWORD} -d username=${USERNAME} -d port=8091
	bucketName := "cfdefault"
	credentials := model.Credential{
		URI:          cburl,
		UserName:     userID,
		Password:     passwd,
		SASLPassword: saslpasswd,
		BucketName:   bucketName,
	}
	utils.Logger.Printf("client.bosh.configureCouchbaseInstance: %v\n", credentials)
	request, err = http.NewRequest("POST",
		fmt.Sprintf("%s/settings/web", cburl),
		strings.NewReader(fmt.Sprintf("username=%s&password=%s&port=%d", credentials.UserName, credentials.Password, cbProps.port)))
	request.SetBasicAuth("Administrator", "password")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	response, err = client.Do(request)
	if err != nil {
		return nil, err
	}
	switch response.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized:
		return nil, fmt.Errorf("Bad response from Couchbase(4): %v", response.StatusCode)
	}
	defer response.Body.Close()

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
	}
	switch response.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized:
		return nil, fmt.Errorf("Bad response from Couchbase(5): %v", response.StatusCode)
	}
	defer response.Body.Close()

	return &credentials, nil

}

func (c *BoshClient) configureCouchbaseCluster(ipaddrs []string, userID, passwd string) (err error) {
	// tell first node about each of the others

	// POST
	// curl -s -u user:pass http://10.254.0.2:8091/controller/addNode -d 'hostname=10.254.0.6&user=USERNAME&password=PASSWORD&services=kv%2Cindex%2Cn1ql'

	client := &http.Client{}
	cburl := fmt.Sprintf("http://%s:%d", ipaddrs[0], 8091)
	utils.Logger.Printf("client.bosh.configureCouchbaseCluster - using base URL: %v\n", cburl)

	var knownNodes = "ns_1%40" + ipaddrs[0]
	for idx, ip := range ipaddrs[1:] {
		utils.Logger.Printf("client.bosh.configureCouchbaseCluster - adding node %d: %v\n", idx, ip)
		knownNodes = knownNodes + "%2Cns_1%40" + ip
		reqString := fmt.Sprintf("hostname=%s&user=%s&password=%s&services=%s", ip, userID, passwd, "kv%2Cindex%2Cn1ql")
		request, err := http.NewRequest("POST", fmt.Sprintf("%s/controller/addNode", cburl),
			strings.NewReader(reqString))
		request.SetBasicAuth(userID, passwd)
		request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		utils.Logger.Printf("client.bosh.configureCouchbaseCluster: addNode #%d %s REQUEST\n%s\n\n", idx, ip, c.dumpRequest(request))

		response, err := client.Do(request)
		utils.Logger.Printf("client.bosh.configureCouchbaseCluster: addNode #%d %s RESPONSE\n%s\n\n", idx, ip, c.dumpResponse(response))
		if err != nil {
			utils.Logger.Printf("client.bosh.configureCouchbaseCluster: error in http addNode %s POST: %v\n", ip, err)
			return err
		}
		switch response.StatusCode {
		case http.StatusBadRequest:
			utils.Logger.Printf("client.bosh.configureCouchbaseCluster: got 400 from addNode %d/%s, ignoring\n", idx, ip)
			continue
		case http.StatusOK:
			continue
		default:
			utils.Logger.Printf("client.bosh.configureCouchbaseCluster: bad response code from http addNode POST %v\n", response.StatusCode)
			return errors.New("Bad cluster status code")
		}
	}

	// rebalance: POST /controller/rebalance -d 'ejectedNodes=&knownNodes=ns_1%40192.168.0.77%2Cns_1%40192.168.0.56'
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/controller/rebalance", cburl),
		strings.NewReader(fmt.Sprintf("ejectedNodes=&knownNodes=%s", knownNodes)))
	request.SetBasicAuth(userID, passwd)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	utils.Logger.Printf("client.bosh.configureCouchbaseCluster: rebalance REQUEST: \n%s\n\n", c.dumpRequest(request))
	response, err := client.Do(request)
	utils.Logger.Printf("client.bosh.configureCouchbaseCluster: rebalance RESPONSE: \n%s\n\n", c.dumpResponse(response))
	if err != nil {
		utils.Logger.Printf("client.bosh.configureCouchbaseCluster: error in http rebalance POST %v\n", err)
		return err
	}
	if response.StatusCode != 200 {
		utils.Logger.Printf("client.bosh.configureCouchbaseCluster: bad response code from http rebalance POST %v\n", response.StatusCode)
		return errors.New("Bad rebalance status code")
	}

	return nil
}

func (c *BoshClient) dumpRequest(request *http.Request) string {
	data, err := httputil.DumpRequest(request, true)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	return string(data)
}

func (c *BoshClient) dumpResponse(response *http.Response) string {
	data, err := httputil.DumpResponse(response, true)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	return string(data)
}
