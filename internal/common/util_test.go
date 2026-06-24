package common

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfResource "github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestStringSliceToTFList(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		wantLen  int
		wantNull bool
	}{
		{
			name:     "normal slice",
			input:    []string{"a", "b", "c"},
			wantLen:  3,
			wantNull: false,
		},
		{
			name:     "empty slice",
			input:    []string{},
			wantLen:  0,
			wantNull: false,
		},
		{
			name:     "nil slice",
			input:    nil,
			wantLen:  0,
			wantNull: false,
		},
		{
			name:     "single element",
			input:    []string{"only"},
			wantLen:  1,
			wantNull: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags := StringSliceToTFList(tt.input)

			if diags.HasError() {
				t.Errorf("StringSliceToTFList() returned errors: %v", diags)
			}

			if result.IsNull() != tt.wantNull {
				t.Errorf("StringSliceToTFList() IsNull = %v, want %v", result.IsNull(), tt.wantNull)
			}

			if len(result.Elements()) != tt.wantLen {
				t.Errorf("StringSliceToTFList() len = %d, want %d", len(result.Elements()), tt.wantLen)
			}
		})
	}
}

func TestValidateHTTPStatus(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		acceptedCodes []int
		nilResp       bool
		wantValid     bool
		wantError     bool
	}{
		{
			name:          "nil response rejected",
			nilResp:       true,
			acceptedCodes: []int{http.StatusOK},
			wantValid:     false,
			wantError:     true,
		},
		{
			name:          "status OK accepted",
			statusCode:    http.StatusOK,
			acceptedCodes: []int{http.StatusOK},
			wantValid:     true,
			wantError:     false,
		},
		{
			name:          "status Created accepted",
			statusCode:    http.StatusCreated,
			acceptedCodes: []int{http.StatusOK, http.StatusCreated},
			wantValid:     true,
			wantError:     false,
		},
		{
			name:          "status NotFound rejected",
			statusCode:    http.StatusNotFound,
			acceptedCodes: []int{http.StatusOK},
			wantValid:     false,
			wantError:     true,
		},
		{
			name:          "multiple accepted codes",
			statusCode:    http.StatusNoContent,
			acceptedCodes: []int{http.StatusOK, http.StatusNoContent, http.StatusNotFound},
			wantValid:     true,
			wantError:     false,
		},
		{
			name:          "server error rejected",
			statusCode:    http.StatusInternalServerError,
			acceptedCodes: []int{http.StatusOK, http.StatusCreated},
			wantValid:     false,
			wantError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diagnostics diag.Diagnostics
			var httpResp *http.Response
			if !tt.nilResp {
				httpResp = &http.Response{StatusCode: tt.statusCode}
			}

			result := ValidateHTTPStatus(&diagnostics, httpResp, "test operation", tt.acceptedCodes...)

			if result != tt.wantValid {
				t.Errorf("ValidateHTTPStatus() = %v, want %v", result, tt.wantValid)
			}

			if diagnostics.HasError() != tt.wantError {
				t.Errorf("ValidateHTTPStatus() HasError = %v, want %v", diagnostics.HasError(), tt.wantError)
			}
		})
	}
}

func TestParseResourceIdentifiers(t *testing.T) {
	const (
		resourceUUID        = "11111111-1111-1111-1111-111111111111"
		fallbackProjectUUID = "22222222-2222-2222-2222-222222222222"
		explicitProjectUUID = "33333333-3333-3333-3333-333333333333"
	)

	client := &CrusoeClient{ProjectID: fallbackProjectUUID}

	tests := []struct {
		name         string
		importID     string
		wantResource string
		wantProject  string
		wantErr      bool
	}{
		{
			name:         "resource id only falls back to client project",
			importID:     resourceUUID,
			wantResource: resourceUUID,
			wantProject:  fallbackProjectUUID,
			wantErr:      false,
		},
		{
			name:         "explicit project id from suffix",
			importID:     resourceUUID + "," + explicitProjectUUID,
			wantResource: resourceUUID,
			wantProject:  explicitProjectUUID,
			wantErr:      false,
		},
		{
			name:     "invalid resource uuid",
			importID: "not-a-uuid",
			wantErr:  true,
		},
		{
			name:     "invalid project uuid in suffix",
			importID: resourceUUID + ",not-a-uuid",
			wantErr:  true,
		},
		{
			name:     "too many comma separated parts",
			importID: resourceUUID + "," + explicitProjectUUID + "," + fallbackProjectUUID,
			wantErr:  true,
		},
		{
			name:     "empty import id",
			importID: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tfResource.ImportStateRequest{ID: tt.importID}

			resourceID, projectID, errMsg := ParseResourceIdentifiers(req, client, "resource_id")

			if tt.wantErr {
				if errMsg == "" {
					t.Errorf("ParseResourceIdentifiers(%q) expected error, got none", tt.importID)
				}

				return
			}

			if errMsg != "" {
				t.Errorf("ParseResourceIdentifiers(%q) unexpected error: %s", tt.importID, errMsg)
			}

			if resourceID != tt.wantResource {
				t.Errorf("ParseResourceIdentifiers(%q) resourceID = %q, want %q", tt.importID, resourceID, tt.wantResource)
			}

			if projectID != tt.wantProject {
				t.Errorf("ParseResourceIdentifiers(%q) projectID = %q, want %q", tt.importID, projectID, tt.wantProject)
			}
		})
	}
}
