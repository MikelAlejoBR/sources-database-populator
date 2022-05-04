package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MikelAlejoBR/sources-database-populator/config"
	"github.com/MikelAlejoBR/sources-database-populator/logger"
	"github.com/MikelAlejoBR/sources-database-populator/source_types_db"
	"github.com/RedHatInsights/sources-api-go/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// availabilityStatuses holds the availability statuses that a resource might be in.
var availabilityStatuses = [4]string{
	"available",
	"in_progress",
	"partially_available",
	"unavailable",
}

// endpointAvailabilityStatuses holds the availability statuses that an endpoint might be in.
var endpointAvailabilityStatuses = [2]string{
	"available",
	"unavailable",
}

// appCreationWorkflows holds the app creation workflows of a source_types_db.
var appCreationWorkflows = [2]string{
	"account_authorization",
	"manual_configuration",
}

// These variables will hold the total count of the created resources.
var (
	createdApplicationsTotal    uint64
	createdAuthenticationsTotal uint64
	createdEndpointsTotal       uint64
	createdRhcConnectionsTotal  uint64
	createdSourcesTotal         uint64
)

// sourceTypesDb is the access to the in-memory database we will be using to store the different source types,
// application types and their compatible authorization types.
var sourceTypesDb = source_types_db.SourceTypesDb{}

// IdStruct is a helper struct to extract IDs from creation requests.
type IdStruct struct {
	Id string `json:"id"`
}

func main() {
	// Parse all the configuration to get the required environment variables.
	config.ParseConfig()

	// Initialize the zap logger.
	logger.InitializeLogger()

	// Call the health check endpoint to confirm that the back end is up and running.
	performHealthCheck()

	// Initialize the in memory database.
	sourceTypesDb.InitializeDatabase()

	// Get the time before starting the process so that we can calculate the elapsed time afterwards.
	startTs := time.Now()

	// Start the process.
	for _, tenant := range config.Tenants {
		var wg sync.WaitGroup
		for i := 0; i < config.SourcesPerTenant; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				sourceId, sourceTypeId, ok := createSource(tenant)
				if !ok {
					return
				}

				createApplications(tenant, sourceTypeId, sourceId)
				createAuthenticationsSource(tenant, sourceTypeId, sourceTypeId)
				createEndpoints(tenant, sourceId)
				createRhcConnections(tenant, sourceId)

				atomic.AddUint64(&createdSourcesTotal, 1)
			}()
		}

		wg.Wait()
	}

	// Calculate the elapsed time.
	elapsedTime := time.Now().Sub(startTs).String()

	logger.Logger.Infow(
		"Statistics - created resources",
		zap.String("elapsed_time", elapsedTime),
		zap.Uint64("created_sources", createdSourcesTotal),
		zap.Uint64("created_endpoints", createdEndpointsTotal),
		zap.Uint64("created_applications", createdApplicationsTotal),
		zap.Uint64("created_authentications", createdAuthenticationsTotal),
		zap.Uint64("created_rhc_connections", createdRhcConnectionsTotal),
	)

	// Make sure we flush the buffer from any logs.
	logger.FlushLoggingBuffer()
}

// performHealthCheck sends a request to the back end's "/health" endpoint to check that it is online.
func performHealthCheck() {
	// Before proceeding, send a request to the health check endpoint to be sure that the back end is running.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.SourcesApiHealthUrl, nil)
	if err != nil {
		logger.Logger.Fatalw(
			"could not create the health check request",
			zap.Error(err),
			zap.String("health_check_url", config.SourcesApiHealthUrl),
		)
	}

	req.Header.Set("Accept", "application/json")
	// A "x-rh-identity" with an "account number: 12345"
	req.Header.Set("x-rh-identity", "ewogICAgImlkZW50aXR5IjogewogICAgICAgICJhY2NvdW50X251bWJlciI6ICIxMjM0NSIKICAgIH0KfQ==")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Logger.Fatalw(
			"could not send the health check request",
			zap.Error(err),
		)
	}

	if err := res.Body.Close(); err != nil {
		logger.Logger.Fatalw(
			"could not close the body of the response of the health check request",
			zap.Error(err),
		)
	}

	if res.StatusCode != http.StatusOK {
		logger.Logger.Fatalw(
			"unexpected status code received from the health check",
			zap.Int("want_satus_code", http.StatusOK),
			zap.Int("got_status-code", res.StatusCode),
		)
	}
}

// getRandomAppCreationWorkflow returns a random app creation workflow.
func getRandomAppCreationWorkflow() string {
	idx := rand.Intn(1)

	return appCreationWorkflows[idx]
}

// getRandomAvailabilityStatus returns a random availability status.
func getRandomAvailabilityStatus() string {
	idx := rand.Intn(3)

	return availabilityStatuses[idx]
}

// getRandomEndpointAvailabilityStatus returns a random availability status for endpoints.
func getRandomEndpointAvailabilityStatus() string {
	idx := rand.Intn(2)

	return endpointAvailabilityStatuses[idx]
}

// createSource takes a target tenant and creates random sources and sub resources. That is: authentications, endpoints,
// applications and rhc connections.
func createSource(tenant string) (string, string, bool) {
	st := sourceTypesDb.GetRandomSourceType()

	uid, err := uuid.NewUUID()
	if err != nil {
		logger.Logger.Errorw(`could not generate UUID when generating a source. Skipping...`, zap.Error(err))
		return "", "", false
	}

	name := fmt.Sprintf("%s-name", uid)
	uidStr := uid.String()
	source := model.SourceCreateRequest{
		Name:                &name,
		Uid:                 &uidStr,
		AppCreationWorkflow: getRandomAppCreationWorkflow(),
		AvailabilityStatus:  getRandomAvailabilityStatus(),
		SourceTypeIDRaw:     st.Id,
	}

	body, err := json.Marshal(source)
	if err != nil {
		logger.Logger.Errorw(
			`could not marshal "SourceCreateRequest" into JSON. Skipping...`,
			zap.Error(err),
			zap.Any("source_create_request", source),
		)
		return "", "", false
	}

	resBody, isSuccess := sendCreationRequest("source", tenant, config.SourceCreateUrl, body)
	if !isSuccess {
		return "", "", false
	}

	var sourceId IdStruct
	err = json.Unmarshal(resBody, &sourceId)
	if err != nil {
		logger.Logger.Errorw(
			"could not extract ID from source creation response. Can not create subresources, skipping...",
			zap.Error(err),
			zap.String("tenant", tenant),
			zap.Any("request_body", json.RawMessage(body)),
			zap.Any("response_body", json.RawMessage(resBody)),
		)
		return "", "", false
	}

	logger.Logger.Debugw(
		"Source created",
		zap.String("tenant", tenant),
		zap.Any("response_body", json.RawMessage(resBody)),
	)
	logger.Logger.Infow(
		"Source created",
		zap.String("id", sourceId.Id),
	)

	return sourceId.Id, st.Id, true
}

// createRhcConnections creates rhc connections related to the given source.
func createRhcConnections(tenant string, sourceId string) {
	var wg sync.WaitGroup

	for i := 0; i < config.RhcConnectionsPerTenant; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			uid, err := uuid.NewUUID()
			if err != nil {
				logger.Logger.Errorw("could not generate UUID when generating a rhc connection. Skipping...", zap.Error(err))
				return
			}

			rhcConnection := model.RhcConnectionCreateRequest{
				RhcId:       uid.String(),
				SourceIdRaw: sourceId,
			}

			body, err := json.Marshal(rhcConnection)
			if err != nil {
				logger.Logger.Errorw(
					`could not marshal "RhcConnectionCreateRequest" into JSON. Skipping...`,
					zap.Error(err),
					zap.Any("rhc_connection_create_request", rhcConnection),
				)
				return
			}

			resBody, isSuccess := sendCreationRequest("rhcConnection", tenant, config.RhcConnectionCreateUrl, body)
			if !isSuccess {
				return
			}

			var rhcConnectionId IdStruct
			err = json.Unmarshal(resBody, &rhcConnectionId)
			if err != nil {
				logger.Logger.Errorw(
					"could not extract ID from rhc connection creation response. Skipping...",
					zap.Error(err),
					zap.String("tenant", tenant),
					zap.Any("request_body", json.RawMessage(body)),
					zap.Any("response_body", json.RawMessage(resBody)),
				)
				return
			}

			logger.Logger.Debugw(
				"RHC Connection created",
				zap.String("tenant", tenant),
				zap.Any("response_body", json.RawMessage(resBody)),
			)
			logger.Logger.Infow(
				"RHC connection created",
				zap.String("id", rhcConnectionId.Id),
			)

			atomic.AddUint64(&createdRhcConnectionsTotal, 1)
		}()
	}

	wg.Wait()
}

// createEndpoints creates the endpoints related to the given source.
func createEndpoints(tenant string, sourceId string) {
	var wg sync.WaitGroup
	for i := 0; i < config.EndpointsPerSource; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			uid, err := uuid.NewUUID()
			if err != nil {
				logger.Logger.Errorw(`could not generate UUID when generating an endpoint. Skipping...`, zap.Error(err))
				return
			}

			endpoint := model.EndpointCreateRequest{
				AvailabilityStatus: getRandomEndpointAvailabilityStatus(),
				Host:               fmt.Sprintf("source-%s.com", sourceId),
				Path:               fmt.Sprintf("/source-%s", sourceId),
				Role:               uid.String(),
				SourceIDRaw:        sourceId,
			}

			body, err := json.Marshal(endpoint)
			if err != nil {
				logger.Logger.Errorw(
					`could not marshal "EndpointCreateRequest" into JSON. Skipping...`,
					zap.Error(err),
					zap.Any("endpoint_create_request", endpoint),
				)
				return
			}

			resBody, isSuccess := sendCreationRequest("endpoint", tenant, config.EndpointCreateUrl, body)
			if !isSuccess {
				return
			}

			var endpointId IdStruct
			err = json.Unmarshal(resBody, &endpointId)
			if err != nil {
				logger.Logger.Errorw(
					"could not extract ID from endpoint creation response. Skipping...",
					zap.Error(err),
					zap.String("tenant", tenant),
					zap.Any("request_body", json.RawMessage(body)),
					zap.Any("response_body", json.RawMessage(resBody)),
				)
				return
			}

			logger.Logger.Debugw(
				"Endpoint created",
				zap.String("tenant", tenant),
				zap.String("source_id", sourceId),
				zap.Any("response_body", json.RawMessage(resBody)),
			)
			logger.Logger.Infow(
				"Endpoint created",
				zap.String("id", endpointId.Id),
			)

			atomic.AddUint64(&createdEndpointsTotal, 1)
		}()
	}

	wg.Wait()
}

// createAuthenticationsSource creates authentications for the given source. It makes sure to create compatible
// authentications for that source.
func createAuthenticationsSource(tenant string, sourceTypeId string, sourceId string) {
	var wg sync.WaitGroup
	for i := 0; i < config.AuthenticationsPerResource; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			authType := sourceTypesDb.GetRandomAuthenticationTypeForSource(sourceTypeId)

			isSuccess := createAuthentications(tenant, authType, "Source", sourceId)
			if !isSuccess {
				return
			}

			atomic.AddUint64(&createdAuthenticationsTotal, 1)
		}()
	}

	wg.Wait()
}

// createAuthenticationsApplication creates authentications for the given application. It makes sure to create
// authentications that are compatible with the application in the given source.
func createAuthenticationsApplication(tenant string, sourceTypeId string, applicationTypeId, applicationId string) {
	var wg sync.WaitGroup
	for i := 0; i < config.AuthenticationsPerResource; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			authType := sourceTypesDb.GetRandomAuthenticationTypeForApplication(sourceTypeId, applicationTypeId)

			isSuccess := createAuthentications(tenant, authType, "Application", applicationId)
			if !isSuccess {
				return
			}

			atomic.AddUint64(&createdAuthenticationsTotal, 1)
		}()
	}

	wg.Wait()
}

// createAuthentications is a generic function which creates authentications for the specified resource type and
// resource id.
func createAuthentications(tenant string, authType string, resourceType string, resourceId string) bool {
	uid, err := uuid.NewUUID()
	if err != nil {
		logger.Logger.Errorw("could not generate UUID when generating an authentication. Skipping...", zap.Error(err))
		return false
	}

	name := fmt.Sprintf("%s-name", uid)
	username := fmt.Sprintf("%s-username", uid)
	password := fmt.Sprintf("%s-password", uid)
	authentication := model.AuthenticationCreateRequest{
		AuthType:      authType,
		Name:          &name,
		Password:      &password,
		ResourceType:  resourceType,
		ResourceIDRaw: resourceId,
		Username:      &username,
	}

	body, err := json.Marshal(authentication)
	if err != nil {
		logger.Logger.Errorw(
			`could not marshal "AuthenticationCreateRequest" into JSON. Skipping...`,
			zap.Error(err),
			zap.Any("authentication_create_request", authentication),
		)
		return false
	}

	resBody, isSuccess := sendCreationRequest("authentication", tenant, config.AuthenticationCreateUrl, body)
	if !isSuccess {
		return false
	}

	var authenticationId IdStruct
	err = json.Unmarshal(resBody, &authenticationId)
	if err != nil {
		logger.Logger.Errorw(
			"could not extract ID from authentication creation response. Skipping...",
			zap.Error(err),
			zap.String("tenant", tenant),
			zap.Any("request_body", json.RawMessage(body)),
			zap.Any("response_body", json.RawMessage(resBody)),
		)
		return false
	}

	logger.Logger.Debugw(
		"Authentication creation's response body",
		zap.String("tenant_id", tenant),
		zap.Any("response_body", resBody),
	)
	logger.Logger.Infow(
		"Authentication created",
		zap.String("authentication_id", authenticationId.Id),
	)

	return true
}

// createApplications creates the application and its authentications which are compatible with the provided source.
func createApplications(tenant string, sourceTypeId string, sourceId string) {
	var wg sync.WaitGroup
	for i := 0; i < config.ApplicationsPerSource; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			appType := sourceTypesDb.GetRandomApplicationType(sourceTypeId)

			application := model.ApplicationCreateRequest{
				ApplicationTypeIDRaw: appType.Id,
				SourceIDRaw:          sourceId,
			}

			body, err := json.Marshal(application)
			if err != nil {
				logger.Logger.Errorw(
					`could not marshal "ApplicationCreateRequest" into JSON. Skipping...`,
					zap.Error(err),
					zap.Any("application_create_request", application),
				)
				return
			}

			resBody, isSuccess := sendCreationRequest("application", tenant, config.ApplicationCreateUrl, body)
			if !isSuccess {
				return
			}

			var applicationId IdStruct
			err = json.Unmarshal(resBody, &applicationId)
			if err != nil {
				logger.Logger.Errorw(
					"could not extract ID from application creation response. Can not create authentications, skipping...",
					zap.Error(err),
					zap.Any("response_body", json.RawMessage(resBody)),
				)
				return
			}

			logger.Logger.Debugw(
				"Application creation's response body",
				zap.String("tenant_id", tenant),
				zap.String("source_id", sourceId),
				zap.Any("response_body", json.RawMessage(resBody)),
			)
			logger.Logger.Infow(
				"Application created",
				zap.String("application_id", applicationId.Id),
			)

			atomic.AddUint64(&createdApplicationsTotal, 1)

			go createAuthenticationsApplication(tenant, sourceTypeId, appType.Id, applicationId.Id)
		}()
	}

	wg.Wait()
}

// sendCreationRequest is a generic function which sends a resource creation request to the back end.
func sendCreationRequest(resourceType string, tenant string, url string, body []byte) ([]byte, bool) {
	// We use a channel as the throttler for the number of simultaneous requests. Each new process will write to the
	// channel, "allocating a slot" to perform the request. Once the request is done, the process will read from the
	// channel, "freeing the slot" so that other processes can perform their requests. If the channel is full of
	// values, the process will block here until some other process frees a slot.
	config.ConcurrentRequests <- struct{}{}

	logger.Logger.Debugw(
		"Request parameters for the creation request",
		zap.String("resource_type", resourceType),
		zap.String("tenant", tenant),
		zap.String("url", url),
		zap.Any("body", json.RawMessage(body)),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		logger.Logger.Errorw(
			"could not create request for the resource creation. Skipping...",
			zap.Error(err),
			zap.String("resource_type", resourceType),
			zap.String("tenant", tenant),
			zap.String("url", url),
			zap.Any("body", json.RawMessage(body)),
		)

		<-config.ConcurrentRequests
		return nil, false
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("x-rh-identity", tenant)

	logger.Logger.Debugw("Request to be sent", zap.Any("request", req))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Logger.Errorw(
			"could not send the creation request. Skipping...",
			zap.Error(err),
			zap.String("resource_type", resourceType),
			zap.String("tenant", tenant),
			zap.String("url", url),
			zap.Any("body", json.RawMessage(body)),
		)

		<-config.ConcurrentRequests
		return nil, false
	}

	// Request is done, we can free one slot in the channel.
	<-config.ConcurrentRequests

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Logger.Errorw(
			"could not read the resource creation's response body",
			zap.Error(err),
			zap.String("resource_type", resourceType),
			zap.String("tenant", tenant),
			zap.String("url", url),
			zap.Any("body", json.RawMessage(body)),
		)
	}
	if err = res.Body.Close(); err != nil {
		logger.Logger.Errorw(
			"could not close the resource creation's response body",
			zap.Error(err),
			zap.String("resource_type", resourceType),
			zap.String("tenant", tenant),
			zap.String("url", url),
			zap.Any("body", json.RawMessage(body)),
		)
	}

	if res.StatusCode != http.StatusCreated {
		logger.Logger.Errorw(
			"unexpected status code when creating a resource. Skipping...",
			zap.Int("want_status_code", http.StatusOK),
			zap.Any("response_body", json.RawMessage(resBody)),
			zap.Int("got_status_code", res.StatusCode),
			zap.String("resource_type", resourceType),
			zap.String("tenant", tenant),
			zap.String("url", url),
			zap.Any("body", json.RawMessage(body)),
		)
		return nil, false
	}

	return resBody, true
}
