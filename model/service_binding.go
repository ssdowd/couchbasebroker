package model

// A Credential holds information about credentials for a Couchbase instance.
type Credential struct {
	URI          string `json:"couchbase_url"`
	UserName     string `json:"username"`
	Password     string `json:"password"`
	SASLPassword string `json:"saslpassword"`
	BucketName   string `json:"bucket"`
}

// A ServiceBinding holds information about a binding between an app and a service.
type ServiceBinding struct {
	ID                string `json:"id"`
	ServiceID         string `json:"service_id"`
	AppID             string `json:"app_id"`
	ServicePlanID     string `json:"service_plan_id"`
	ServiceInstanceID string `json:"service_instance_id"`
	Credential
}

// A CreateServiceBindingResponse contains credentials for a binding.
type CreateServiceBindingResponse struct {
	// SyslogDrainUrl string      `json:"syslog_drain_url, omitempty"`
	Credentials interface{} `json:"credentials"`
}
