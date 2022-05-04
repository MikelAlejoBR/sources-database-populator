package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/google/uuid"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

// defaultApplicationsPerSource is the default number of applications that will be created per source_types_db.
const defaultApplicationsPerSource = 10

// defaultAuthenticationsPerResource is the default number of authentications that will be created per resource.
const defaultAuthenticationsPerResource = 3

// defaultConcurrentRequests is the default number of requests that the program is allowed to send at the same time.
const defaultConcurrentRequests = 10

// defaultEndpointsPerSource is the default number of endpoints that will be created per source_types_db.
const defaultEndpointsPerSource = 10

// defaultRhcConnectionsPerTenant is the default number of rhcConnections that will be created per source_types_db.
const defaultRhcConnectionsPerTenant = 10

// defaultSourcesPerTenant is the default number of sources that will be created per tenant.
const defaultSourcesPerTenant = 10

// defaultTenants is the default number of tenants that will be created.
const defaultTenants = 3

// sourcesV31Path is the path to the latest API version.
const sourcesV31Path = "api/sources/v3.1"

// ApplicationsPerSource is the number of applications the program will create for each source_types_db.
var ApplicationsPerSource int

// AuthenticationsPerResource is the number of authentications the program will create for each resource.
var AuthenticationsPerResource int

// ConcurrentRequests is the maximum number of concurrent requests that the program is allowed to send at the same time.
var ConcurrentRequests chan struct{}

// EndpointsPerSource is the number of endpoints the program will create for each source_types_db.
var EndpointsPerSource int

// LogLevel is the log level the logger will be configured at.
var LogLevel string

// RhcConnectionsPerTenant is the number of rhcConnections the program will create for each tenant.
var RhcConnectionsPerTenant int

// SourcesApiHealthUrl is the full URL for the "health" endpoint of the sources-api back end.
var SourcesApiHealthUrl string

// SourcesApiUrl is the URL for the sources-api back end, including the "v31Path".
var SourcesApiUrl string

// SourcesPerTenant is the number of sources the program will create for each tenant.
var SourcesPerTenant int

// Tenants holds an array of base64 XRHID objects with random OrgIds ready to be sent to the back end.
var Tenants []string

// URLs for the different endpoints we will be sending requests to.
var (
	ApplicationCreateUrl    string
	ApplicationTypesUrl     string
	AuthenticationCreateUrl string
	EndpointCreateUrl       string
	RhcConnectionCreateUrl  string
	SourceCreateUrl         string
	SourceTypesUrl          string
)

// ParseConfig grabs the URL for the Sources API instance and the parameters to create the fixtures on the database.
func ParseConfig() {
	// Get the log level for the logger.
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		LogLevel = "info"
	} else {
		LogLevel = logLevel
	}

	// Get the sources instance's host.
	sourcesHost := os.Getenv("SOURCES_API_HOST")
	if sourcesHost == "" {
		log.Fatalf("configuration missing: Sources API host")
	}

	// Get the sources instance's port.
	sourcesPort := os.Getenv("SOURCES_API_PORT")
	if sourcesPort == "" || sourcesPort == "0" {
		log.Fatalf("configuration missing: Sources API port")
	}

	// Build the URL.
	SourcesApiHealthUrl = fmt.Sprintf("%s:%s/health", sourcesHost, sourcesPort)
	SourcesApiUrl = fmt.Sprintf("%s:%s/%s", sourcesHost, sourcesPort, sourcesV31Path)

	// Get the maximum number of concurrent requests.
	concurrentRequests := os.Getenv("CONCURRENT_REQUESTS")
	if concurrentRequests == "" {
		ConcurrentRequests = make(chan struct{}, defaultConcurrentRequests)
	} else {
		tmp, err := strconv.Atoi(concurrentRequests)
		if err != nil {
			log.Fatalf(`could not parse the maximum concurrent requests for the program: %s`, err)
		}

		if tmp < 1 {
			log.Printf(`warning: you specified less than 1 concurrent requests: %d. Defaulting to %d`, tmp, defaultConcurrentRequests)
			ConcurrentRequests = make(chan struct{}, defaultConcurrentRequests)
		} else {
			ConcurrentRequests = make(chan struct{}, tmp)
		}
	}

	// Get the number of tenants to be created.
	numberTenants := os.Getenv("NUMBER_OF_TENANTS")
	var tenantsNumber int
	if numberTenants == "" {
		tenantsNumber = defaultTenants
	} else {
		tmp, err := strconv.Atoi(numberTenants)
		if err != nil {
			log.Fatalf(`could not parse the number of tenants to create: %s`, err)
		}

		tenantsNumber = tmp
	}

	// Generate the XRHID objects with random OrgIds.
	var xRhIds []identity.XRHID
	for i := 0; i < tenantsNumber; i++ {
		id, err := uuid.NewUUID()
		if err != nil {
			log.Fatalf(`could not generate UUID for the default tenants: %s`, err)
		}

		xRhIds = append(
			xRhIds,
			identity.XRHID{
				Identity: identity.Identity{
					AccountNumber: id.String(),
				},
			},
		)
	}

	// Transform the XRHID objects to base64 encoded strings ready to be used in the "x-rh-identity" headers.
	for _, xRhId := range xRhIds {
		result, err := json.Marshal(xRhId)
		if err != nil {
			log.Fatalf(`could not JSON encode the XRHID object: %s`, err)
		}

		Tenants = append(Tenants, base64.StdEncoding.EncodeToString(result))
	}

	// Get the sources to create per tenant.
	sourcesPerTenant := os.Getenv("SOURCES_PER_TENANT")
	if sourcesPerTenant == "" {
		SourcesPerTenant = defaultSourcesPerTenant
	} else {
		tmp, err := strconv.Atoi(sourcesPerTenant)
		if err != nil {
			log.Fatalf(`could not parse the number of sources to create per tenant: %s`, err)
		}

		SourcesPerTenant = tmp
	}

	// Get the rhcConnections to create per tenant.
	rhcConnectionsPerTenant := os.Getenv("RHC_CONNECTIONS_PER_TENANT")
	if rhcConnectionsPerTenant == "" {
		RhcConnectionsPerTenant = defaultRhcConnectionsPerTenant
	} else {
		tmp, err := strconv.Atoi(rhcConnectionsPerTenant)
		if err != nil {
			log.Fatalf(`could not parse the number of rhc connections to create per tenant: %s`, err)
		}

		RhcConnectionsPerTenant = tmp
	}

	// Get the applications to create per source_types_db.
	applicationsPerSource := os.Getenv("APPLICATIONS_PER_SOURCE")
	if applicationsPerSource == "" {
		ApplicationsPerSource = defaultApplicationsPerSource
	} else {
		tmp, err := strconv.Atoi(applicationsPerSource)
		if err != nil {
			log.Fatalf(`could not parse the number of applications to create per source_types_db: %s`, err)
		}

		ApplicationsPerSource = tmp
	}

	// Get the endpoints to create per source_types_db.
	endpointsPerSource := os.Getenv("ENDPOINTS_PER_SOURCE")
	if endpointsPerSource == "" {
		EndpointsPerSource = defaultEndpointsPerSource
	} else {
		tmp, err := strconv.Atoi(endpointsPerSource)
		if err != nil {
			log.Fatalf(`could not parse the number of endpoints to create per source_types_db: %s`, err)
		}

		EndpointsPerSource = tmp
	}

	authenticationsPerResource := os.Getenv("AUTHENTICATIONS_PER_RESOURCE")
	if authenticationsPerResource == "" {
		AuthenticationsPerResource = defaultAuthenticationsPerResource
	} else {
		tmp, err := strconv.Atoi(authenticationsPerResource)
		if err != nil {
			log.Fatalf(`could not parse the number of authentications to create per resource: %s`, err)
		}

		AuthenticationsPerResource = tmp
	}

	// Initialize the endpoint URLs we will be sending the requests to.
	ApplicationCreateUrl = fmt.Sprintf("%s/applications", SourcesApiUrl)
	ApplicationTypesUrl = fmt.Sprintf("%s/application_types", SourcesApiUrl)
	AuthenticationCreateUrl = fmt.Sprintf("%s/authentications", SourcesApiUrl)
	EndpointCreateUrl = fmt.Sprintf("%s/endpoints", SourcesApiUrl)
	RhcConnectionCreateUrl = fmt.Sprintf("%s/rhc_connections", SourcesApiUrl)
	SourceCreateUrl = fmt.Sprintf("%s/sources", SourcesApiUrl)
	SourceTypesUrl = fmt.Sprintf("%s/source_types", SourcesApiUrl)
}
