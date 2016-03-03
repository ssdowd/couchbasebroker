package model

// A Service contains information about a service corresponding to a catalog entry.
type Service struct {
	Name           string   `json:"name"`
	ID             string   `json:"id"`
	Description    string   `json:"description"`
	Bindable       bool     `json:"bindable"`
	PlanUpdateable bool     `json:"plan_updateable, omitempty"`
	Tags           []string `json:"tags, omitempty"`
	Requires       []string `json:"requires, omitempty"`

	Metadata        interface{}   `json:"metadata, omitempty"`
	Plans           []ServicePlan `json:"plans"`
	DashboardClient interface{}   `json:"dashboard_client"`
}
