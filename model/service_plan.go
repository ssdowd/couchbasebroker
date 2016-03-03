package model

// A ServicePlan contains information about a service plan (description, name, Metadata, price).
type ServicePlan struct {
	Name        string      `json:"name"`
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Metadata    interface{} `json:"metadata, omitempty"`
	Free        bool        `json:"free, omitempty"`
}
