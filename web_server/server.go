package web_server

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	// "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"

	"github.com/ssdowd/couchbasebroker/config"
	"github.com/ssdowd/couchbasebroker/model"
	"github.com/ssdowd/couchbasebroker/utils"
)

var (
	conf = config.GetConfig()
)

type Server struct {
	controller *Controller
}

func CreateServer(cloudName string, cloudOptionsFile string) (*Server, error) {
	serviceInstances, err := loadServiceInstances()
	if err != nil {
		utils.Logger.Printf("CreateServer error from loadServiceInstances: %v\n", err)
		return nil, err
	}

	serviceBindings, err := loadServiceBindings()
	if err != nil {
		utils.Logger.Printf("CreateServer error from loadServiceBindings: %v\n", err)
		return nil, err
	}
	utils.Logger.Printf("CreateServer cloudName:'%s', cloudOptionsFile:'%s'\n\tserviceInstances:'%v', serviceBindings:'%v'\n", cloudName, cloudOptionsFile, serviceInstances, serviceBindings)

	controller, err := CreateController(cloudName, cloudOptionsFile, serviceInstances, serviceBindings)
	if err != nil {
		utils.Logger.Printf("CreateServer error from CreateController: %v\n", err)
		return nil, err
	}

	return &Server{
		controller: controller,
	}, nil
}

func (s *Server) Start() {
	router := mux.NewRouter()

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !checkAuth(req) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, req)
		})
	}

	router.HandleFunc("/v2/catalog", s.controller.Catalog).Methods("GET")
	router.HandleFunc("/v2/service_instances/{service_instance_guid}", s.controller.GetServiceInstance).Methods("GET")
	router.HandleFunc("/v2/service_instances/{service_instance_guid}", s.controller.CreateServiceInstance).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{service_instance_guid}", s.controller.RemoveServiceInstance).Methods("DELETE")
	router.HandleFunc("/v2/service_instances/{service_instance_guid}/last_operation", s.controller.GetServiceInstance).Methods("GET")
	router.HandleFunc("/v2/service_instances/{service_instance_guid}/service_bindings/{service_binding_guid}", s.controller.Bind).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{service_instance_guid}/service_bindings/{service_binding_guid}", s.controller.UnBind).Methods("DELETE")

	http.Handle("/", middleware(router))

	cfPort := os.Getenv("PORT")
	if cfPort != "" {
		conf.Port = cfPort
	}

	fmt.Println("Server started, listening on port " + conf.Port + "...")
	fmt.Println("CTL-C to break out of broker")
	http.ListenAndServe(":"+conf.Port, nil)
}

// private methods

func loadServiceInstances() (map[string]*model.ServiceInstance, error) {
	var serviceInstancesMap map[string]*model.ServiceInstance

	err := utils.ReadAndUnmarshal(&serviceInstancesMap, conf.DataPath, conf.ServiceInstancesFileName)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("WARNING: service instance data file '%s' does not exist\n", conf.ServiceInstancesFileName)
			fmt.Printf("WARNING: service instance data path is '%s'\n", conf.DataPath)
			serviceInstancesMap = make(map[string]*model.ServiceInstance)
		} else {
			return nil, errors.New(fmt.Sprintf("Could not load the service instances, message: %s", err.Error()))
		}
	}

	return serviceInstancesMap, nil
}

func loadServiceBindings() (map[string]*model.ServiceBinding, error) {
	var bindingMap map[string]*model.ServiceBinding

	err := utils.ReadAndUnmarshal(&bindingMap, conf.DataPath, conf.ServiceBindingsFileName)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("WARNING: key map data file '%s' does not exist\n", conf.ServiceBindingsFileName)
			bindingMap = make(map[string]*model.ServiceBinding)
		} else {
			return nil, errors.New(fmt.Sprintf("Could not load the service instances, message: %s", err.Error()))
		}
	}

	return bindingMap, nil
}

func checkAuth(r *http.Request) bool {
	user, pass, _ := r.BasicAuth()
	if user == "" || user != conf.RestUser || pass != conf.RestPassword {
		return false
	}
	return true
}
