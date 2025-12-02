package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/antihax/optional"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	tfResource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
)

const (
	pollInterval = 2 * time.Second

	ErrorMsgProviderInitFailed = "Could not initialize the Crusoe provider." +
		" Please check your Crusoe configuration and try again, and if the problem persists, contact support@crusoecloud.com."

	latestVersionURL   = "https://api.github.com/repos/crusoecloud/terraform-provider-crusoe/releases/latest"
	colorGreen         = "\033[32m"
	colorYellow        = "\033[33m"
	colorRed           = "\033[31m"
	colorReset         = "\033[0m"
	metadataFile       = "/.crusoe/.metadata"
	DevelopmentMessage = "This feature is currently in development. Reach out to support@crusoecloud.com with any questions."
	onlyUserReadPerms  = 0o600
	two                = 2
	internalErrorCode  = "internal_error"
)

var version string

func GetVersion() string {
	if version == "" {
		return "v0.0.0-unspecified"
	}

	return version
}

type opStatus string

type opResultError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var (
	OpSucceeded  opStatus = "SUCCEEDED"
	OpInProgress opStatus = "IN_PROGRESS"
	OpFailed     opStatus = "FAILED"

	ErrUnableToGetOpRes = errors.New("failed to get result of operation")

	// fallback error presented to the user in unexpected situations
	errUnexpected = errors.New("An unexpected error occurred. Please try again, and if the problem persists, contact support@crusoecloud.com.")

	// error messages
	errBadMapCast  = errors.New("failed to cast tf map value to string")
	errBadListCast = errors.New("failed to cast tf list value to string")
)

type Metadata struct {
	VersionCheckDate string `json:"versionCheckDate"`
}

// NewAPIClient initializes a new Crusoe API client with the given configuration.
func NewAPIClient(host, key, secret string) *swagger.APIClient {
	cfg := swagger.NewConfiguration()
	cfg.UserAgent = fmt.Sprintf("CrusoeTerraform/%s", version)
	cfg.BasePath = host
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = buildRetryClient().StandardClient()
	}

	cfg.HTTPClient.Transport = NewAuthenticatingTransport(cfg.HTTPClient.Transport, key, secret)

	return swagger.NewAPIClient(cfg)
}

// AwaitOperation polls an async API operation until it resolves into a success or failure state.
func AwaitOperation(ctx context.Context, op *swagger.Operation, projectID string,
	getFunc func(context.Context, string, string) (swagger.Operation, *http.Response, error)) (
	*swagger.Operation, error,
) {
	for op.State == string(OpInProgress) {
		updatedOps, httpResp, err := getFunc(ctx, projectID, op.OperationId)
		if err != nil {
			return nil, fmt.Errorf("error getting operation with id %s: %w", op.OperationId, err)
		}
		httpResp.Body.Close()

		op = &updatedOps

		time.Sleep(pollInterval)
	}

	switch op.State {
	case string(OpSucceeded):
		return op, nil
	case string(OpFailed):
		opError, err := opResultToError(op.Result)
		if err != nil {
			return op, err
		}

		return op, opError
	default:

		return op, errUnexpected
	}
}

// AwaitOperationAndResolve awaits an async API operation and attempts to parse the response as an instance of T,
// if the operation was successful.
func AwaitOperationAndResolve[T any](ctx context.Context, op *swagger.Operation, projectID string,
	getFunc func(context.Context, string, string) (swagger.Operation, *http.Response, error),
) (*T, *swagger.Operation, error) {
	op, err := AwaitOperation(ctx, op, projectID, getFunc)
	if err != nil {
		return nil, op, err
	}

	result, err := parseOpResult[T](op.Result)
	if err != nil {
		return nil, op, err
	}

	return result, op, nil
}

type CrusoeClient struct {
	APIClient *swagger.APIClient
	ProjectID string
}

func GetProjectIDOrFallback(client *CrusoeClient, projectId string) string {
	if projectId != "" {
		return projectId
	}

	return client.ProjectID
}

func GetProjectIDFromPointerOrFallback(client *CrusoeClient, projectId *string) string {
	projectIdStr := ""
	if projectId != nil {
		projectIdStr = *projectId
	}

	return GetProjectIDOrFallback(client, projectIdStr)
}

func ParseResourceIdentifiers(req tfResource.ImportStateRequest, client *CrusoeClient, resourceIDFieldName string) (resourceID, projectID, err string) {
	// We allow "{resourceIDFieldName}" (implicit project_id via env variable) or "{resourceIDFieldName},project_id" (explicit project_id)
	resourceIdentifiers := strings.Split(req.ID, ",")

	if (len(resourceIdentifiers) != 1) && (len(resourceIdentifiers) != 2) {
		return "", "", fmt.Sprintf("Expected format %s,project_id, got %q", resourceIDFieldName, req.ID)
	}

	resourceID = resourceIdentifiers[0]
	projectID = client.ProjectID
	if len(resourceIdentifiers) == 2 {
		projectID = resourceIdentifiers[1]
	}

	if _, parseErr := uuid.Parse(resourceID); parseErr != nil {
		return "", "", fmt.Sprintf("Failed to parse %s: %v", resourceIDFieldName, parseErr)
	}

	if _, parseErr := uuid.Parse(projectID); parseErr != nil {
		return "", "", fmt.Sprintf("Failed to parse project ID: %v", parseErr)
	}

	return resourceID, projectID, ""
}

func parseOpResult[T any](opResult interface{}) (*T, error) {
	b, err := json.Marshal(opResult)
	if err != nil {
		return nil, ErrUnableToGetOpRes
	}

	var result T
	err = json.Unmarshal(b, &result)
	if err != nil {
		return nil, ErrUnableToGetOpRes
	}

	return &result, nil
}

func opResultToError(res interface{}) (expectedErr, unexpectedErr error) {
	b, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal operation error: %w", err)
	}
	resultError := opResultError{}
	err = json.Unmarshal(b, &resultError)
	if err != nil {
		return nil, fmt.Errorf("op result type not error as expected: %w", err)
	}

	//nolint:goerr113 //This function is designed to return dynamic errors
	return fmt.Errorf("%s", resultError.Message), nil
}

// UnpackAPIError takes a swagger API error and safely attempts to extract any additional information
// present in the response. The original error is returned unchanged if it cannot be unpacked.
func UnpackAPIError(original error) error {
	apiErr := &swagger.GenericSwaggerError{}
	if ok := errors.As(original, apiErr); !ok {
		return original
	}

	var model swagger.ErrorBody
	err := json.Unmarshal(apiErr.Body(), &model)
	if err != nil {
		return original
	}

	// some error messages are of the format "rpc code = ... desc = ..."
	// in those cases, we extract the description and return it
	errorMsg := model.Message
	components := strings.Split(model.Message, " desc = ")
	if len(components) == two {
		errorMsg = components[1]
	}

	if model.Code == internalErrorCode && model.ErrorId != "" {
		errorMsg = fmt.Sprintf("%s. Error ID: %s.", errorMsg, model.ErrorId)
	}

	//nolint:goerr113 // error is dynamic
	return fmt.Errorf("%s", errorMsg)
}

// GetUpdateMessageIfValid checks if the current terraform provider version is up-to-date with the latest release and
// returns a banner if the version needs an update. A new check is only performed if the last one
// was over 24 hours ago.
//
//nolint:cyclop,nestif,govet // breaking up function would hurt readability
func GetUpdateMessageIfValid(ctx context.Context) string {
	metadata := Metadata{}

	// Parse metadata file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	metadataFilePath := homeDir + metadataFile

	_, fileErr := os.Stat(metadataFilePath)
	if os.IsNotExist(fileErr) {
		// create a new file in the config directory to store metadata
		_, err := os.Create(metadataFilePath)
		if err != nil {
			return ""
		}
	} else {
		fileContent, err := os.ReadFile(metadataFilePath)
		if err != nil {
			return ""
		}
		err = json.Unmarshal(fileContent, &metadata)
		if err != nil {
			return ""
		}
		versionCheckDate, err := time.Parse(time.RFC3339, metadata.VersionCheckDate)
		if err != nil {
			return ""
		}
		// do not check again if version was checked within a day
		if time.Since(versionCheckDate) < time.Hour*24 {
			return ""
		}
	}

	latestVersion, err := getLatestVersion(ctx)
	if err != nil {
		return ""
	}
	currentVersion := GetVersion()

	metadata.VersionCheckDate = time.Now().UTC().Format(time.RFC3339)
	b, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	err = os.WriteFile(metadataFilePath, b, onlyUserReadPerms)
	if err != nil {
		return ""
	}

	if currentVersion < latestVersion {
		return FormatUpdateMessage(currentVersion, latestVersion)
	}

	return ""
}

func getLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestVersionURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("unable to get latest version: %w", err)
	}
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to get latest version: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to get latest version: %w", err)
	}
	resp.Body.Close()
	bodyStruct := make(map[string]interface{})
	err = json.Unmarshal(body, &bodyStruct)
	if err != nil {
		return "", fmt.Errorf("unable to get latest version: %w", err)
	}
	latestVersion := fmt.Sprintf("%v", bodyStruct["tag_name"])

	return latestVersion, nil
}

func FormatUpdateMessage(currentVersion, latestVersion string) string {
	// use red if major version update needed
	if strings.Split(currentVersion, ".")[0] < strings.Split(latestVersion, ".")[0] {
		currentVersion = colorRed + currentVersion + colorReset
	} else {
		currentVersion = colorYellow + currentVersion + colorReset
	}
	latestVersion = colorGreen + latestVersion + colorReset
	body := fmt.Sprintf("    Update available: %s -> %s    ", currentVersion, latestVersion)
	border := ""
	emptyLine := ""
	colorLen := len(colorGreen + colorReset)
	for i := 0; i < len(body)-2*colorLen; i++ { // len(body) includes color escape sequences
		border += "─"
		emptyLine += " "
	}
	body = "│" + body + "│"
	emptyLine = "│" + emptyLine + "│"
	msg := fmt.Sprintf("\n┌%s┐\n%s\n%s\n%s\n└%s┘\n", border, emptyLine, body, emptyLine, border)

	return msg
}

// FindResourceArgs are used to generalize the pattern of iterating through projects to find a resource.
type FindResourceArgs[T any] struct {
	ResourceID string
	// A function which performs the API operation
	GetResource func(ctx context.Context, projectId string, resourceID string) (
		T, *http.Response, error)
	// A function which checks that the resource is the resource being found
	IsResource func(T, string) bool
}

func FindResource[T any](ctx context.Context, client *swagger.APIClient, args FindResourceArgs[T]) (
	resource *T, projectID string, err error,
) {
	opts := &swagger.ProjectsApiListProjectsOpts{
		OrgId: optional.EmptyString(),
	}

	projectsResp, projectHttpResp, err := client.ProjectsApi.ListProjects(ctx, opts)

	defer projectHttpResp.Body.Close()

	if err != nil {
		return nil, "", fmt.Errorf("failed to query for projects: %w", err)
	}

	for _, project := range projectsResp.Items {
		resource, getResourceHttpResp, getResourceErr := args.GetResource(ctx, project.Id, args.ResourceID)
		if getResourceErr != nil {
			continue
		}
		if args.IsResource(resource, args.ResourceID) {
			return &resource, project.Id, nil
		}
		getResourceHttpResp.Body.Close()
	}

	return nil, "", errors.New("failed to find resource")
}

func TFMapToStringMap(tfMap types.Map) (map[string]string, error) {
	// Convert the Terraform map to a string map
	stringMap := make(map[string]string)

	for key, val := range tfMap.Elements() {
		stringVal, ok := val.(types.String)
		if !ok {
			return nil, errBadMapCast
		}
		stringMap[key] = stringVal.ValueString()
	}

	return stringMap, nil
}

func TFListToStringSlice(tfList types.List) ([]string, error) {
	// Convert the Terraform list to a string slice
	stringSlice := make([]string, len(tfList.Elements()))

	for i, val := range tfList.Elements() {
		stringVal, ok := val.(types.String)
		if !ok {
			return nil, errBadListCast
		}
		stringSlice[i] = stringVal.ValueString()
	}

	return stringSlice, nil
}

func StringerSliceToTFList[T fmt.Stringer](s []T) (types.List, diag.Diagnostics) {
	tfList := make([]attr.Value, len(s))
	for i, val := range s {
		tfList[i] = types.StringValue(val.String())
	}

	return types.ListValue(types.StringType, tfList)
}

func StringSliceToTFList(s []string) (types.List, diag.Diagnostics) {
	tfList := make([]attr.Value, len(s))
	for i, val := range s {
		tfList[i] = types.StringValue(val)
	}

	return types.ListValue(types.StringType, tfList)
}

func StringerMapToTFMap[T fmt.Stringer](m map[string]T) (types.Map, diag.Diagnostics) {
	tfMap := make(map[string]attr.Value)
	for key, val := range m {
		tfMap[key] = types.StringValue(val.String())
	}

	return types.MapValue(types.StringType, tfMap)
}

func StringMapToTFMap(m map[string]string) (types.Map, diag.Diagnostics) {
	tfMap := make(map[string]attr.Value)
	for key, val := range m {
		tfMap[key] = types.StringValue(val)
	}

	return types.MapValue(types.StringType, tfMap)
}

func AddProjectError(resp *provider.ConfigureResponse, defaultProject, titleIfEmpty, msgIfEmpty, titleIfSet, msgIfSet string) {
	title := titleIfEmpty
	msg := msgIfEmpty
	if defaultProject != "" {
		title = titleIfSet
		msg = msgIfSet
	}
	resp.Diagnostics.AddAttributeError(path.Root("default_project"), title, msg)
}
