package client

import (
	model "github.com/ssdowd/couchbasebroker/model"
)

// A Client implements the connection to some type of IaaS to provide services via a service broker.
type Client interface {
	CreateInstance(parameters interface{}) (string, error)
	GetInstanceState(instanceID string) (string, error)
	DeleteInstance(instanceID string) error

	// new interface
	GetCredentials(instanceID string) (*model.Credential, error)
	RemoveCredentials(instanceID string, bindingID string) error

	// old SSH to a VM interface
	InjectKeyPair(instanceID string) (string, string, string, error)
	RevokeKeyPair(instanceID string, privateKey string) error

	SetCatalog(catalog *model.Catalog) error
	GetCatalog() *model.Catalog
	IsValidPlan(planName string) bool
}
