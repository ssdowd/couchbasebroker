package model

type ServiceInstance struct {
	Id               string `json:"id"`
	DashboardUrl     string `json:"dashboard_url"`
	InternalId       string `json:"internalId, omitempty"`
	ServiceId        string `json:"service_id"`
	PlanId           string `json:"plan_id"`
	OrganizationGuid string `json:"organization_guid"`
	SpaceGuid        string `json:"space_guid"`

	LastOperation *LastOperation `json:"last_operation, omitempty"`

	Parameters interface{} `json:"parameters, omitempty"`

	Credential Credential
	// Credential interface{} `json:"credentials, omitempty"`
}

type LastOperation struct {
	State                    string `json:"state"`
	Description              string `json:"description"`
	DashboardUrl             string `json:"dashboard_url"`
	AsyncPollIntervalSeconds int    `json:"async_poll_interval_seconds, omitempty"`
}

type CreateServiceInstanceResponse struct {
	DashboardUrl  string         `json:"dashboard_url"`
	LastOperation *LastOperation `json:"last_operation, omitempty"`
}

type Message struct {
	Description string `json:"description"`
}
