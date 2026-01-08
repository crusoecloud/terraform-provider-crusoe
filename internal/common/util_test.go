package common

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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
		wantValid     bool
		wantError     bool
	}{
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
			httpResp := &http.Response{StatusCode: tt.statusCode}

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
