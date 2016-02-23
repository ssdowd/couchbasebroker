package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	conf "github.com/ssdowd/couchbasebroker/config"
	utils "github.com/ssdowd/couchbasebroker/utils"
	webs "github.com/ssdowd/couchbasebroker/web_server"
)

type Options struct {
	ConfigPath       string
	Cloud            string
	CloudOptionsPath string
}

var options Options

func init() {
	defaultConfigPath := utils.GetPath([]string{"assets", "config.json"})
	defaultCloudOptsPath := utils.GetPath([]string{"assets", "boshconfig.json"})
	flag.StringVar(&options.ConfigPath, "config", defaultConfigPath, "use '--config' option to specify the config file path")

	flag.StringVar(&options.Cloud, "service", utils.BOSH, "use '--service' option to specify the cloud client to use: DOCKER, Bosh, AWS or SoftLayer (SL)")

	flag.StringVar(&options.CloudOptionsPath, "copts", defaultCloudOptsPath, "use '--copts' option to specify the specific cloud options file")

	flag.Parse()
}

func main() {
	err := checkCloudName(options.Cloud)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	utils.Logger.Printf("Cloud: %v\n", options.Cloud)
	utils.Logger.Printf("Config Path: %v\n", options.ConfigPath)

	_, err = conf.LoadConfig(options.ConfigPath)
	if err != nil {
		panic(fmt.Sprintf("Error loading config file [%v]...", err))
	}

	server, err := webs.CreateServer(options.Cloud, options.CloudOptionsPath)
	if err != nil {
		panic(fmt.Sprintf("Error creating server [%v]...", err))
	}

	server.Start()
}

// Private func

func checkCloudName(name string) error {
	switch name {
	case utils.DOCKER, utils.AWS, utils.SOFTLAYER, utils.SL, utils.BOSH:
		return nil
	}

	return errors.New(fmt.Sprintf("Invalid cloud name: %s", name))
}
