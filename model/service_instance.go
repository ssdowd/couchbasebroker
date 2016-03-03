package model

// A ServiceInstance contains information about a created service.
type ServiceInstance struct {
	ID               string `json:"id"`
	DashboardURL     string `json:"dashboard_url"`
	InternalID       string `json:"internalId, omitempty"`
	ServiceID        string `json:"service_id"`
	PlanID           string `json:"plan_id"`
	OrganizationGUID string `json:"organization_guid"`
	SpaceGUID        string `json:"space_guid"`

	LastOperation *LastOperation `json:"last_operation, omitempty"`

	Parameters interface{} `json:"parameters, omitempty"`

	Credential Credential
	// Credential interface{} `json:"credentials, omitempty"`
}

// A LastOperation contains information about the state of a service instance.
type LastOperation struct {
	State                    string `json:"state"`
	Description              string `json:"description"`
	DashboardURL             string `json:"dashboard_url"`
	AsyncPollIntervalSeconds int    `json:"async_poll_interval_seconds, omitempty"`
}

// A CreateServiceInstanceResponse contains information about created service instance.
type CreateServiceInstanceResponse struct {
	DashboardURL  string         `json:"dashboard_url"`
	LastOperation *LastOperation `json:"last_operation, omitempty"`
}

// A Message is a generic message object to return over REST as JSON.
type Message struct {
	Description string `json:"description"`
}
