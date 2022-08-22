package source_types_db

import (
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/MikelAlejoBR/sources-database-populator/config"
	"github.com/MikelAlejoBR/sources-database-populator/logger"
	"go.uber.org/zap"
)

// sourceNameId holds the name of the source type and its corresponding id, for an easy and quick lookup of "get the ID
// of this source name".
var sourceNameId = make(map[string]string)

// sourceTypes "is the database" which contains all the source types with their corresponding authentications and
// compatible applications.
var sourceTypes = make(map[string]SourceType)

// sourceTypesKeys is a helper list which will allow us getting random source types easier.
var sourceTypesKeys []string

// SourceType is the structure we will use to store the source type, its compatible authentications, its compatible
// applications, and the compatible authentications for those applications.
type SourceType struct {
	Id                         string
	Name                       string
	CompatibleAuthentications  []string
	CompatibleApplicationTypes map[string]ApplicationType
}

// ApplicationType holds the structure for an application and its compatible authentication types.
type ApplicationType struct {
	Id                        string
	CompatibleAuthentications []string
}

// SourceTypesDb is the structure we will use for accessing the database.
type SourceTypesDb struct{}

// CreateSourceType creates a new source type from the "sourceTypeId" and the "sourceTypeName".
func (sdb SourceTypesDb) CreateSourceType(sourceTypeId string, sourceTypeName string) {
	// Store the source type name and id for an easier lookup afterwards. This will be useful when receiving the
	// "get application types" response, where we will the sources they are compatible with by their name. That way it
	// will be easier to link them to source ids.
	sourceNameId[sourceTypeName] = sourceTypeId

	sourceTypes[sourceTypeId] = SourceType{
		Id:   sourceTypeId,
		Name: sourceTypeName,
	}
}

// AddAuthenticationType adds a compatible authentication type to the given source type id.
func (sdb SourceTypesDb) AddAuthenticationType(sourceTypeId string, authenticationType string) {
	// Get the source type.
	sourceType := sourceTypes[sourceTypeId]

	// Append the authentication type.
	sourceType.CompatibleAuthentications = append(sourceType.CompatibleAuthentications, authenticationType)

	// Make sure to overwrite the source type as otherwise it won't be saved.
	sourceTypes[sourceTypeId] = sourceType
}

// AddCompatibleApplicationType adds the application type ID to all the compatible source types of the database. It
// also adds the supported authentication types as compatible authentications for the application.
func (sdb SourceTypesDb) AddCompatibleApplicationType(applicationTypeId string, supportedSourceTypes []string, supportedAuthenticationTypes map[string][]string) {
	for _, sst := range supportedSourceTypes {
		// Fetch the source type id by its name.
		sstId := sourceNameId[sst]

		// Fetch the source type by its ID.
		sourceType := sourceTypes[sstId]
		// In order to avoid nil pointer dereferences, make sure that the map is at least initialized.
		if sourceType.CompatibleApplicationTypes == nil {
			sourceType.CompatibleApplicationTypes = make(map[string]ApplicationType)
		}

		// The application type might already have been added to the source type. In that case append the
		// authentications to the ones that already existed.
		appType, ok := sourceType.CompatibleApplicationTypes[applicationTypeId]
		if ok {
			appType.CompatibleAuthentications = append(appType.CompatibleAuthentications, supportedAuthenticationTypes[sourceType.Name]...)

			// Overwrite the source type to store the changes.
			sourceTypes[sstId] = sourceType
		} else {
			// Create the brand new application type.
			sourceType.CompatibleApplicationTypes[applicationTypeId] = ApplicationType{
				Id:                        applicationTypeId,
				CompatibleAuthentications: supportedAuthenticationTypes[sourceType.Name],
			}

			// Overwrite the source type to store the changes.
			sourceTypes[sstId] = sourceType
		}
	}
}

// GetRandomAuthenticationTypeForApplication gets a random authentication type that is compatible with the provided
// application type id, which in turn is compatible with the provided source type id as well.
func (sdb SourceTypesDb) GetRandomAuthenticationTypeForApplication(sourceTypeId string, applicationTypeId string) string {
	st := sourceTypes[sourceTypeId]
	appTypes := st.CompatibleApplicationTypes[applicationTypeId]

	// The "azure" and "google" source types from the cloud meter application don't have a defined authentication, so
	// in this case we can return a fixed authentication type.
	if appTypes.CompatibleAuthentications == nil {
		return "cloud-meter-app-does-not-have-azure-or-google-supported-authentication-types"
	}
	idx := rand.Intn(len(appTypes.CompatibleAuthentications))

	return appTypes.CompatibleAuthentications[idx]
}

// GetRandomAuthenticationTypeForSource gets a random compatible authentication type for the given source type id.
func (sdb SourceTypesDb) GetRandomAuthenticationTypeForSource(sourceTypeId string) string {
	st := sourceTypes[sourceTypeId]

	idx := rand.Intn(len(st.CompatibleAuthentications))

	return st.CompatibleAuthentications[idx]
}

// GetApplicationTypes returns the list of the compatible application types for the given source.
func (sdb SourceTypesDb) GetApplicationTypes(sourceTypeId string) []ApplicationType {
	st := sourceTypes[sourceTypeId]

	var applicationTypes = make([]ApplicationType, 0, len(st.CompatibleApplicationTypes))
	for _, appType := range st.CompatibleApplicationTypes {
		applicationTypes = append(applicationTypes, appType)
	}

	return applicationTypes
}

// GetRandomSourceType returns a random source type from the database.
func (sdb SourceTypesDb) GetRandomSourceType() SourceType {
	// Initialize the array if it is not initialized already.
	if sourceTypesKeys == nil {
		for key := range sourceTypes {
			sourceTypesKeys = append(sourceTypesKeys, key)
		}
	}

	// Get a random index for the keys array.
	randomIdx := rand.Intn(len(sourceTypesKeys))

	// Get a random key from the keys array.
	randomKey := sourceTypesKeys[randomIdx]

	// Return a random source type.
	return sourceTypes[randomKey]
}

func (sdb SourceTypesDb) InitializeDatabase() {
	getSourceTypes()
	getApplicationTypes()
}

// getSourceTypes sends a request to fetch all the source types and stores them in the database.
func getSourceTypes() {
	// Three seconds is more than enough to hit the API and receive a response. We don't have thousands of source types.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.SourceTypesUrl, nil)
	if err != nil {
		logger.Logger.Fatalw(
			"could not create the request for the source types",
			zap.Error(err),
			zap.Any("request", req),
		)
	}

	req.Header.Set("Accept", "application/json")
	// A "x-rh-identity" with an "account number: 12345"
	req.Header.Set("x-rh-identity", "ewogICAgImlkZW50aXR5IjogewogICAgICAgICJhY2NvdW50X251bWJlciI6ICIxMjM0NSIKICAgIH0KfQ==")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Logger.Fatalw(
			"could not get the source types",
			zap.Error(err),
			zap.Any("request", req),
			zap.Any("response", resp),
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Logger.Fatalw(`could not read the body from the "get source types" response`,
			zap.Error(err),
			zap.Any("request", req),
			zap.Any("response", resp),
		)
	}

	if err := resp.Body.Close(); err != nil {
		logger.Logger.Fatalw(`could not close the body from the "get source types" response`,
			zap.Error(err),
			zap.Any("request", req),
			zap.Any("response", resp),
		)
	}

	// Temporary struct since the JSON comes in the following format: {"data": [{...}, {...}, {...}]}. We cannot
	// marshal the payload into a "[]model.SourceType" because the IDs come as strings and the model has an int id.
	// Therefore, the "Unmarshal" function fails.
	type Auth struct {
		Type string `json:"type"`
	}

	type Schema struct {
		Authentication []Auth `json:"authentication"`
	}

	type SourceType struct {
		Id     string `json:"id"`
		Name   string `json:"name"`
		Schema Schema `json:"schema"`
	}

	type SourceTypeResponse struct {
		SourceTypes []SourceType `json:"data"`
	}

	// Unmarshal the JSON response into the above structs.
	var responseBody SourceTypeResponse
	if err := json.Unmarshal(body, &responseBody); err != nil {
		logger.Logger.Fatalw(
			"could not marshal the source types to a struct",
			zap.Error(err),
			zap.Any("response_body", responseBody),
		)
	}

	for _, st := range responseBody.SourceTypes {
		// We don't want to store the "rh-marketplace" source_types_db for the moment since it doesn't have any compatible apps
		// or authorizations for it.
		if st.Name == "rh-marketplace" {
			continue
		}

		// Create all the source types and their compatible authentication types.
		SourceTypesDb{}.CreateSourceType(st.Id, st.Name)
		for _, auth := range st.Schema.Authentication {
			SourceTypesDb{}.AddAuthenticationType(st.Id, auth.Type)
		}
	}
}

// getApplicationTypes sends a request to the back end to fetch all the application types, and then it relates them to
// the existing source types from the database.
func getApplicationTypes() {
	// Three seconds is more than enough to hit the API and receive a response. We don't have thousands of application
	// types.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.ApplicationTypesUrl, nil)
	if err != nil {
		logger.Logger.Fatalw(
			"could not create the request for the application types",
			zap.Error(err),
			zap.Any("request", req),
		)
	}

	req.Header.Set("Accept", "application/json")
	// A "x-rh-identity" with an "account number: 12345"
	req.Header.Set("x-rh-identity", "ewogICAgImlkZW50aXR5IjogewogICAgICAgICJhY2NvdW50X251bWJlciI6ICIxMjM0NSIKICAgIH0KfQ==")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Logger.Fatalw(
			"could not get the application types",
			zap.Error(err),
			zap.Any("request", req),
			zap.Any("response", resp),
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Logger.Fatalw(`could not read the body from the "get source types" response`,
			zap.Error(err),
			zap.Any("request", req),
			zap.Any("response", resp),
		)
	}

	if err := resp.Body.Close(); err != nil {
		logger.Logger.Fatalw(`could not close the body from the "get application types" response`,
			zap.Error(err),
			zap.Any("request", req),
			zap.Any("response", resp),
		)
	}

	// Temporary structs to be able to read the response, since it comes in the {"data": [{...}]} form.
	type AppType struct {
		Id                           string              `json:"id"`
		Name                         string              `json:"name"`
		SupportedSourceTypes         []string            `json:"supported_source_types"`
		SupportedAuthenticationTypes map[string][]string `json:"supported_authentication_types"`
	}

	type AppTypeResponse struct {
		AppTypes []AppType `json:"data"`
	}

	// Unmarshal the JSON response into the above structs.
	var responseBody AppTypeResponse
	if err := json.Unmarshal(body, &responseBody); err != nil {
		logger.Logger.Fatalw(
			"could not marshal application types to struct",
			zap.Error(err),
			zap.Any("response_body", responseBody),
		)
	}

	// Add all the compatible application types to the already existing source types. Also store the compatible
	// authentication types for those applications.
	for _, appType := range responseBody.AppTypes {
		SourceTypesDb{}.AddCompatibleApplicationType(appType.Id, appType.SupportedSourceTypes, appType.SupportedAuthenticationTypes)
	}
}
