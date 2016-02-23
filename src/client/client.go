package client

import (
	model "github.com/ssdowd/couchbasebroker/model"
)
  
type Client interface {
	CreateInstance(parameters interface{}) (string, error)
	GetInstanceState(instanceId string) (string, error)
	DeleteInstance(instanceId string) error

  // new interface
  GetCredentials(instanceId string) (*model.Credential,error)
	RemoveCredentials(instanceId string, bindingId string) error
  
  // old SSH to a VM interface
	InjectKeyPair(instanceId string) (string, string, string, error)
	RevokeKeyPair(instanceId string, privateKey string) error
  
  SetCatalog(catalog *model.Catalog) error
  GetCatalog() *model.Catalog
  IsValidPlan(planName string) bool
}
