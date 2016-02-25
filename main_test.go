package main

import (
	"encoding/json"
	"fmt"
	"github.com/ssdowd/couchbasebroker/config"
	"github.com/ssdowd/couchbasebroker/utils"
	"github.com/ssdowd/couchbasebroker/web_server"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
)

var opt = Options{
	ConfigPath:       "assets/config.json",
	Cloud:            "Bosh",
	CloudOptionsPath: "assets/bosh-config.json",
}

var myconfig *config.Config

func TestValidCloud(t *testing.T) {
	err := checkCloudName(opt.Cloud)
	if err != nil {
		t.Errorf("Failed valid cloud name: %v", err)
	}
	err = checkCloudName(utils.AWS)
	if err != nil {
		t.Errorf("Failed valid cloud name: %v", err)
	}
	badName := "Bogus Cloud Name"
	err = checkCloudName(badName)
	if err == nil {
		t.Errorf("Failed to detect invalid cloud name: %v", badName)
	}
}

func TestLoadOptions(t *testing.T) {
	t.Log("Testing load", opt.ConfigPath)
	c, err := config.LoadConfig(opt.ConfigPath)
	if err != nil {
		t.Errorf("Failed load config: (%s): %v", opt.ConfigPath, err)
	}
	myconfig = c
	t.Log("Loaded config", myconfig)
}

func TestCreateServer(t *testing.T) {
	// test stuff here...
	t.Log("Creating Server", opt.Cloud)
	server, err := web_server.CreateServer(opt.Cloud, opt.CloudOptionsPath)
	if err != nil {
		t.Errorf("Failed to create server: %v", err)
	}
	go server.Start()
}

func TestServerListening(t *testing.T) {
	// test stuff here...
	t.Log("Testing Server")
	_, err := net.Dial("tcp", "localhost:"+config.GetConfig().Port)
	if err != nil {
		t.Errorf("Failed to connect to server: %v", err)
	}
}

func TestGetCatalog(t *testing.T) {
	url := "http://localhost:" + config.GetConfig().Port + "/v2/catalog"
	t.Log("Testing GET", url)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(myconfig.RestUser, myconfig.RestPassword)

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("Failed to GET catalog: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("Failed to GET catalog, status: %d", res.StatusCode)
	}

	jsonBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Errorf("Failed reading catalog json: %v", err)
	}
	t.Log("Catalog:", fmt.Sprintf("%s", jsonBytes))

	var f interface{}
	err = json.Unmarshal(jsonBytes, &f)
	if err != nil {
		t.Errorf("Failed unmarshalling catalog json: %v", err)
	}
}
