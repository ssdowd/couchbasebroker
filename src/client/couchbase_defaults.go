package client

type cbDefaultSettings struct {
	adminUser     string
	adminPass     string
	ramQuota      int
	indexRAMQuota int
	dbType        string
	port          int
}

func cbDefaultProps() cbDefaultSettings {
	return cbDefaultSettings{
		adminUser:     "Administrator",
		adminPass:     "password",
		ramQuota:      768,
		indexRAMQuota: 256,
		dbType:        "couchbase",
		port:          8091,
	}
}
