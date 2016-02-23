package model


type Credential struct {
	URI           string `json:"couchbase_url"`
	UserName      string `json:"username"`
	Password      string `json:"password"`
  SASLPassword  string `json:"saslpassword"`
  BucketName    string `json:"bucket"`
}

type ServiceBinding struct {
	Id                string `json:"id"`
	ServiceId         string `json:"service_id"`
	AppId             string `json:"app_id"`
	ServicePlanId     string `json:"service_plan_id"`
	ServiceInstanceId string `json:"service_instance_id"`
  Credential
}

type CreateServiceBindingResponse struct {
	// SyslogDrainUrl string      `json:"syslog_drain_url, omitempty"`
	Credentials interface{} `json:"credentials"`
}
