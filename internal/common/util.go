package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/antihax/optional"
	diag "github.com/hashicorp/terraform-plugin-framework/diag"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
)

const (
	// TODO: pull from config set during build
	version = "v0.5.2"

	pollInterval = 2 * time.Second

	ErrorMsgProviderInitFailed = "Could not initialize the Crusoe provider." +
		" Please check your Crusoe configuration and try again, and if the problem persists, contact support@crusoecloud.com."
)

type opStatus string

type opResultError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var (
	OpSucceeded  opStatus = "SUCCEEDED"
	OpInProgress opStatus = "IN_PROGRESS"
	OpFailed     opStatus = "FAILED"

	errUnableToGetOpRes = errors.New("failed to get result of operation")

	// fallback error presented to the user in unexpected situations
	errMultipleProjects = errors.New("User has multiple projects. Please specify a project to be used.")
	errUnexpected       = errors.New("An unexpected error occurred. Please try again, and if the problem persists, contact support@crusoecloud.com.")
)

// NewAPIClient initializes a new Crusoe API client with the given configuration.
func NewAPIClient(host, key, secret string) *swagger.APIClient {
	cfg := swagger.NewConfiguration()
	cfg.UserAgent = fmt.Sprintf("CrusoeTerraform/%s", version)
	cfg.BasePath = host
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
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

// GetFallbackProject queries the API to get the list of projects belonging to the
// logged in user. If there is one project belonging to the user, it returns that project
// else it adds an error to the diagnostics and returns.
func GetFallbackProject(ctx context.Context, client *swagger.APIClient, diagg *diag.Diagnostics) (string, error) {
	config, err := GetConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get config: %w", err)
	}

	opts := &swagger.ProjectsApiListProjectsOpts{
		OrgId: optional.EmptyString(),
	}

	if config.DefaultProject != "" {
		opts.ProjectName = optional.NewString(config.DefaultProject)
	}

	dataResp, httpResp, err := client.ProjectsApi.ListProjects(ctx, opts)

	defer httpResp.Body.Close()

	if err != nil {
		diagg.AddError("Failed to retrieve project ID",
			"Failed to retrieve project ID for the authenticated user.")

		return "", err
	}

	if len(dataResp.Items) != 1 {
		diagg.AddError("Multiple projects found.",
			"Multiple projects found for the authenticated user. Unable to determine which project to use.")

		return "", errMultipleProjects
	}

	projectID := dataResp.Items[0].Id

	if config.DefaultProject == "" {
		diagg.AddWarning("Default project not specified",
			fmt.Sprintf("A project_id was not specified in the configuration file. "+
				"Please specify a project in the terraform file or set a 'default_project' in your configuration file. "+
				"Falling back to project: %s.", dataResp.Items[0].Name))
	}

	return projectID, nil
}

func parseOpResult[T any](opResult interface{}) (*T, error) {
	b, err := json.Marshal(opResult)
	if err != nil {
		return nil, errUnableToGetOpRes
	}

	var result T
	err = json.Unmarshal(b, &result)
	if err != nil {
		return nil, errUnableToGetOpRes
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

// apiError models the error format returned by the Crusoe API go client.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// UnpackAPIError takes a swagger API error and safely attempts to extract any additional information
// present in the response. The original error is returned unchanged if it cannot be unpacked.
func UnpackAPIError(original error) error {
	apiErr := &swagger.GenericSwaggerError{}
	if ok := errors.As(original, apiErr); !ok {
		return original
	}

	var model apiError
	err := json.Unmarshal(apiErr.Body(), &model)
	if err != nil {
		return original
	}

	// some error messages are of the format "rpc code = ... desc = ..."
	// in those cases, we extract the description and return it
	const two = 2
	components := strings.Split(model.Message, " desc = ")
	if len(components) == two {
		//nolint:goerr113 // error is dynamic
		return fmt.Errorf("%s", components[1])
	}

	//nolint:goerr113 // error is dynamic
	return fmt.Errorf("%s", model.Message)
}
