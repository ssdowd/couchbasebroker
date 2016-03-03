package model

// A Catalog is a container for a set of Services.
type Catalog struct {
	Services []Service `json:"services"`
}
