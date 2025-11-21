// Copyright 2025 Stakater AB
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
	"github.com/stakater/alert-az-do/pkg/alertmanager"
	"github.com/stretchr/testify/require"
)

// Test data structure for template testing
type TestData struct {
	Name   string
	Value  string
	Items  []string
	Labels map[string]string
}

func TestLoadTemplate_ValidFile(t *testing.T) {
	// Create a temporary template file
	tmpDir := t.TempDir()
	templateFile := filepath.Join(tmpDir, "test.tmpl")

	templateContent := `{{ define "test.summary" }}Alert: {{ .Name }}{{ end }}
{{ define "test.description" }}Value: {{ .Value }}{{ end }}`

	err := os.WriteFile(templateFile, []byte(templateContent), 0644)
	require.NoError(t, err)

	logger := log.NewNopLogger()
	tmpl, err := LoadTemplate(templateFile, logger)

	require.NoError(t, err)
	require.NotNil(t, tmpl)
	require.NotNil(t, tmpl.tmpl)
	require.Equal(t, logger, tmpl.logger)
}

func TestLoadTemplate_InvalidFile(t *testing.T) {
	logger := log.NewNopLogger()
	tmpl, err := LoadTemplate("/nonexistent/file.tmpl", logger)

	require.Error(t, err)
	require.Nil(t, tmpl)
}

func TestLoadTemplate_InvalidTemplateContent(t *testing.T) {
	// Create a temporary template file with invalid content
	tmpDir := t.TempDir()
	templateFile := filepath.Join(tmpDir, "invalid.tmpl")

	// Invalid template syntax
	templateContent := `{{ define "test" }}{{ .Name {{ end }}`

	err := os.WriteFile(templateFile, []byte(templateContent), 0644)
	require.NoError(t, err)

	logger := log.NewNopLogger()
	tmpl, err := LoadTemplate(templateFile, logger)

	require.Error(t, err)
	require.Nil(t, tmpl)
}

func TestSimpleTemplate(t *testing.T) {
	tmpl := SimpleTemplate()

	require.NotNil(t, tmpl)
	require.NotNil(t, tmpl.tmpl)
	require.NotNil(t, tmpl.logger)
}

func TestTemplate_Execute_PlainText(t *testing.T) {
	tmpl := SimpleTemplate()

	// Test with plain text (no template syntax)
	result, err := tmpl.Execute("This is plain text", nil)

	require.NoError(t, err)
	require.Equal(t, "This is plain text", result)
}

func TestTemplate_Execute_SimpleTemplate(t *testing.T) {
	tmpl := SimpleTemplate()

	data := TestData{
		Name:  "TestAlert",
		Value: "critical",
	}

	result, err := tmpl.Execute("Alert: {{ .Name }} with severity {{ .Value }}", data)

	require.NoError(t, err)
	require.Equal(t, "Alert: TestAlert with severity critical", result)
}

func TestTemplate_Execute_WithFunctions(t *testing.T) {
	tmpl := SimpleTemplate()

	tests := []struct {
		name     string
		template string
		data     interface{}
		expected string
	}{
		{
			name:     "toUpper function",
			template: "{{ .Name | toUpper }}",
			data:     TestData{Name: "hello"},
			expected: "HELLO",
		},
		{
			name:     "toLower function",
			template: "{{ .Name | toLower }}",
			data:     TestData{Name: "HELLO"},
			expected: "hello",
		},
		{
			name:     "join function",
			template: `{{ join "," .Items }}`,
			data:     TestData{Items: []string{"a", "b", "c"}},
			expected: "a,b,c",
		},
		{
			name:     "stringSlice function",
			template: `{{ stringSlice "a" "b" "c" | join "-" }}`,
			data:     nil,
			expected: "a-b-c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tmpl.Execute(tt.template, tt.data)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplate_Execute_RegexFunctions(t *testing.T) {
	tmpl := SimpleTemplate()

	tests := []struct {
		name     string
		template string
		data     TestData
		expected string
	}{
		{
			name:     "match function - matches",
			template: `{{ if match "^test.*" .Name }}matched{{ else }}not matched{{ end }}`,
			data:     TestData{Name: "testAlert"},
			expected: "matched",
		},
		{
			name:     "match function - no match",
			template: `{{ if match "^prod.*" .Name }}matched{{ else }}not matched{{ end }}`,
			data:     TestData{Name: "testAlert"},
			expected: "not matched",
		},
		{
			name:     "reReplaceAll function",
			template: `{{ reReplaceAll "Alert$" "Issue" .Name }}`,
			data:     TestData{Name: "TestAlert"},
			expected: "TestIssue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tmpl.Execute(tt.template, tt.data)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplate_Execute_GetEnv(t *testing.T) {
	tmpl := SimpleTemplate()

	// Set an environment variable for testing
	testEnvVar := "TEST_TEMPLATE_VAR"
	testValue := "test_value_123"
	err := os.Setenv(testEnvVar, testValue)
	require.NoError(t, err)
	defer func() {
		unsetErr := os.Unsetenv(testEnvVar)
		require.NoError(t, unsetErr)
	}()

	result, err := tmpl.Execute(`{{ getEnv "TEST_TEMPLATE_VAR" }}`, nil)

	require.NoError(t, err)
	require.Equal(t, testValue, result)
}

func TestTemplate_Execute_GetEnv_NotSet(t *testing.T) {
	tmpl := SimpleTemplate()

	result, err := tmpl.Execute(`{{ getEnv "NONEXISTENT_VAR" }}`, nil)

	require.NoError(t, err)
	require.Equal(t, "", result)
}

func TestTemplate_Execute_WithAlertmanagerData(t *testing.T) {
	tmpl := SimpleTemplate()

	data := &alertmanager.Data{
		Status: alertmanager.AlertFiring,
		Alerts: alertmanager.Alerts{
			{
				Status:      alertmanager.AlertFiring,
				Fingerprint: "test-fp",
				Labels: alertmanager.KV{
					"alertname": "HighCPU",
					"severity":  "critical",
					"instance":  "server1",
				},
				Annotations: alertmanager.KV{
					"description": "CPU usage is high",
					"summary":     "High CPU on server1",
				},
			},
		},
		GroupLabels: alertmanager.KV{
			"alertname": "HighCPU",
		},
		CommonLabels: alertmanager.KV{
			"alertname": "HighCPU",
			"severity":  "critical",
		},
		CommonAnnotations: alertmanager.KV{
			"description": "CPU usage is high",
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "alert status",
			template: "[{{ .Status | toUpper }}] Alert",
			expected: "[FIRING] Alert",
		},
		{
			name:     "alert count",
			template: "{{ len .Alerts }} alerts",
			expected: "1 alerts",
		},
		{
			name:     "group labels",
			template: "Alert: {{ .GroupLabels.alertname }}",
			expected: "Alert: HighCPU",
		},
		{
			name:     "common annotations",
			template: "Description: {{ .CommonAnnotations.description }}",
			expected: "Description: CPU usage is high",
		},
		{
			name:     "alert labels with join",
			template: `{{ .CommonLabels.Names | join "," }}`,
			expected: "alertname,severity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tmpl.Execute(tt.template, data)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplate_Execute_InvalidTemplate(t *testing.T) {
	tmpl := SimpleTemplate()

	// Template with syntax error
	_, err := tmpl.Execute("{{ .Name }", TestData{Name: "test"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse template")
}

func TestTemplate_Execute_InvalidField(t *testing.T) {
	tmpl := SimpleTemplate()

	// Access non-existent field in struct should still cause an error
	_, err := tmpl.Execute("{{ .NonExistentField }}", TestData{Name: "test"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "can't evaluate field NonExistentField")
}

func TestTemplate_Execute_ComplexTemplate(t *testing.T) {
	tmpl := SimpleTemplate()

	data := &alertmanager.Data{
		Status: alertmanager.AlertFiring,
		Alerts: alertmanager.Alerts{
			{Status: alertmanager.AlertFiring, Labels: alertmanager.KV{"severity": "critical"}},
			{Status: alertmanager.AlertFiring, Labels: alertmanager.KV{"severity": "warning"}},
		},
		GroupLabels: alertmanager.KV{
			"alertname": "MultipleIssues",
			"cluster":   "prod",
		},
		CommonAnnotations: alertmanager.KV{
			"runbook": "https://runbook.example.com",
		},
	}

	template := `[{{ .Status | toUpper }}:{{ len (.Alerts.Firing) }}] {{ .GroupLabels.alertname }} on {{ .GroupLabels.cluster }}
{{ if .CommonAnnotations.runbook }}Runbook: {{ .CommonAnnotations.runbook }}{{ end }}
Firing alerts: {{ len (.Alerts.Firing) }}`

	result, err := tmpl.Execute(template, data)
	require.NoError(t, err)

	expected := `[FIRING:2] MultipleIssues on prod
Runbook: https://runbook.example.com
Firing alerts: 2`

	require.Equal(t, expected, result)
}

func TestTemplate_Execute_WithDefinedTemplates(t *testing.T) {
	// Create a temporary template file with defined templates
	tmpDir := t.TempDir()
	templateFile := filepath.Join(tmpDir, "azdo.tmpl")

	templateContent := `{{ define "azdo.summary" }}[{{ .Status | toUpper }}] {{ .GroupLabels.alertname }}{{ end }}
{{ define "azdo.description" }}{{ .CommonAnnotations.description }}{{ if .CommonAnnotations.runbook }}
Runbook: {{ .CommonAnnotations.runbook }}{{ end }}{{ end }}`

	err := os.WriteFile(templateFile, []byte(templateContent), 0644)
	require.NoError(t, err)

	logger := log.NewNopLogger()
	tmpl, err := LoadTemplate(templateFile, logger)
	require.NoError(t, err)

	data := &alertmanager.Data{
		Status: alertmanager.AlertFiring,
		GroupLabels: alertmanager.KV{
			"alertname": "DatabaseDown",
		},
		CommonAnnotations: alertmanager.KV{
			"description": "Database is not responding",
			"runbook":     "https://runbook.example.com/db-down",
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "summary template",
			template: `{{ template "azdo.summary" . }}`,
			expected: "[FIRING] DatabaseDown",
		},
		{
			name:     "description template",
			template: `{{ template "azdo.description" . }}`,
			expected: "Database is not responding\nRunbook: https://runbook.example.com/db-down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tmpl.Execute(tt.template, data)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplate_Execute_MissingKeyHandling(t *testing.T) {
	tmpl := SimpleTemplate()

	// Test that missing keys in maps return empty string (not errors) due to missingkey=zero option
	tests := []struct {
		name     string
		template string
		data     map[string]string
		expected string
	}{
		{
			name:     "missing string field in map returns empty string",
			template: "Value: '{{ .missing }}'",
			data:     map[string]string{"present": "value"},
			expected: "Value: ''", // Empty string, not <no value>
		},
		{
			name:     "existing field works normally",
			template: "Value: '{{ .present }}'",
			data:     map[string]string{"present": "value"},
			expected: "Value: 'value'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tmpl.Execute(tt.template, tt.data)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

// Add a separate test for struct field access (which still causes errors)
func TestTemplate_Execute_StructFieldAccess(t *testing.T) {
	tmpl := SimpleTemplate()

	tests := []struct {
		name        string
		template    string
		data        TestData
		shouldError bool
		expected    string
	}{
		{
			name:        "existing field in struct works",
			template:    "Name: {{ .Name }}",
			data:        TestData{Name: "test"},
			shouldError: false,
			expected:    "Name: test",
		},
		{
			name:        "missing field in struct causes error",
			template:    "{{ .NonExistentField }}",
			data:        TestData{Name: "test"},
			shouldError: true,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tmpl.Execute(tt.template, tt.data)
			if tt.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTemplate_Execute_SafeFieldAccess(t *testing.T) {
	tmpl := SimpleTemplate()

	// Test safe ways to handle potentially missing fields with maps (which support missingkey=zero)
	tests := []struct {
		name     string
		template string
		data     alertmanager.KV
		expected string
	}{
		{
			name:     "existing field direct access",
			template: "{{ .severity }}",
			data:     alertmanager.KV{"severity": "critical"},
			expected: "critical",
		},
		{
			name:     "missing field returns empty string",
			template: "Value: '{{ .missing }}'",
			data:     alertmanager.KV{"severity": "critical"},
			expected: "Value: ''",
		},
		{
			name:     "conditional field access with if - missing field",
			template: "{{ if .missing }}Has missing{{ else }}No missing{{ end }}",
			data:     alertmanager.KV{"severity": "critical"},
			expected: "No missing",
		},
		{
			name:     "conditional field access with if - existing field",
			template: "{{ if .severity }}Severity: {{ .severity }}{{ else }}No severity{{ end }}",
			data:     alertmanager.KV{"severity": "warning"},
			expected: "Severity: warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tmpl.Execute(tt.template, tt.data)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplate_Execute_EmptyData(t *testing.T) {
	tmpl := SimpleTemplate()

	result, err := tmpl.Execute("Static text without templating", nil)
	require.NoError(t, err)
	require.Equal(t, "Static text without templating", result)
}

func TestTemplate_Execute_FunctionChaining(t *testing.T) {
	tmpl := SimpleTemplate()

	data := TestData{
		Items: []string{"apple", "banana", "cherry"},
	}

	// Chain multiple functions
	result, err := tmpl.Execute(`{{ .Items | join "," | toUpper }}`, data)
	require.NoError(t, err)
	require.Equal(t, "APPLE,BANANA,CHERRY", result)
}

func TestTemplateFuncs_Coverage(t *testing.T) {
	// Test that all template functions are properly registered
	tmpl := SimpleTemplate()

	// Test all available functions
	tests := []struct {
		name     string
		template string
		data     any
	}{
		{"toUpper", `{{ "hello" | toUpper }}`, nil},
		{"toLower", `{{ "HELLO" | toLower }}`, nil},
		{"contains", `{{ contains "world" "hello world" }}`, nil},
		{"hasPrefix", `{{ hasPrefix "he" "hello" }}`, nil},
		{"hasSuffix", `{{ hasSuffix "lo" "hello" }}`, nil},
		{"title", `{{ title "hello world" }}`, nil},
		{"join", `{{ join "," . }}`, []string{"a", "b"}},
		{"match", `{{ match "test" "test" }}`, nil},
		{"reReplaceAll", `{{ reReplaceAll "a" "b" "cat" }}`, nil},
		{"stringSlice", `{{ stringSlice "one" "two" }}`, nil},
		{"getEnv", `{{ getEnv "PATH" }}`, nil},
		{"toJson", `{{ toJson . }}`, map[string]string{"key": "value"}},
		{"toJsonPretty", `{{ toJsonPretty . }}`, map[string]string{"key": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tmpl.Execute(tt.template, tt.data)
			require.NoError(t, err, "Function %s should be available", tt.name)
		})
	}
}
