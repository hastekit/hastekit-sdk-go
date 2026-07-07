package hastekitgateway

import "net/http"

type Config struct {
	// Endpoint is the URL of the HasteKit Gateway
	Endpoint string

	// VirtualKey is the virtual key for the HasteKit Gateway
	VirtualKey string

	// OrgName is the org name for the HasteKit Gateway
	OrgName string

	// ProjectName is the project name for the HasteKit Gateway
	ProjectName string

	// HttpClient is the HTTP client to use for making requests
	HttpClient *http.Client
}
