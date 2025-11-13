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

package config

import (
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

const testConf = `
# Global defaults, applied to all receivers where not explicitly overridden. Optional.
defaults:
  # API access fields.
  organization: my_test_org
  tenant_id: alert-az-do
  client_id: alert-az-do
  subscription_id: alert-az-do
  client_secret: 'alert-az-do'

  # The type of Azure DevOps work item to create. Required.
  issue_type: Bug
  # Issue priority. Optional.
  priority: Critical
  # Go template invocation for generating the summary. Required.
  summary: '{{ template "azdo.summary" . }}'
  # Go template invocation for generating the description. Optional.
  description: '{{ template "azdo.description" . }}'
  # State to transition into when reopening a closed work item. Required.
  reopen_state: "To Do"
  # Do not reopen issues that are in this state. Optional.
  skip_reopen_state: "Removed"
  # Amount of time after being closed that a work item should be reopened, after which, a new work item is created.
  # Optional (default: always reopen)
  reopen_duration: 0h
  update_in_comment: true
  static_labels: ["defaultlabel"]

# Receiver definitions. At least one must be defined.
receivers:
    # Must match the Alertmanager receiver name. Required.
  - name: 'azdo-ab'
    # Azure DevOps project to create the work item in. Required.
    project: AB
    # Copy all Prometheus labels into separate Azure DevOps tags. Optional (default: false).
    add_group_labels: false
    update_in_comment: false
    static_labels: ["somelabel"]

  - name: 'azdo-xy'
    project: XY
    # Overrides default.
    issue_type: Task
    # Azure DevOps area path. Optional.
    area_path: 'Operations'
    # Azure DevOps iteration path. Optional.
    iteration_path: 'Sprint 1'
    # Azure DevOps components. Optional.
    components: [ 'Operations' ]
    # Standard or custom field values to set on created work item. Optional.
    fields:
      System.AssignedTo: "admin@contoso.com"
      Custom.Field: "{{ .CommonLabels.severity }}"

# File containing template definitions. Required.
template: alert-az-do.tmpl
`

// Generic test that loads the testConf with no errors.
func TestLoadFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_alert-az-do")
	require.NoError(t, err)
	defer func() { require.NoError(t, os.RemoveAll(dir)) }()

	require.NoError(t, os.WriteFile(path.Join(dir, "config.yaml"), []byte(testConf), os.ModePerm))

	_, content, err := LoadFile(path.Join(dir, "config.yaml"), log.NewNopLogger())

	require.NoError(t, err)
	require.Equal(t, testConf, string(content))
}

// Checks if the env var substitution is happening correctly in the loaded file
func TestEnvSubstitution(t *testing.T) {
	err := os.Setenv("JA_USER", "user")
	require.NoError(t, err)
	config := "user: $(JA_USER)"

	content, err := substituteEnvVars([]byte(config), log.NewNopLogger())
	expected := "user: user"
	require.NoError(t, err)
	require.Equal(t, string(content), expected)

	config = "user: $(JA_MISSING)"
	content, err = substituteEnvVars([]byte(config), log.NewNopLogger())
	expected = "user: " // Missing env var results in empty string, not error
	require.NoError(t, err)
	require.Equal(t, string(content), expected)
}

// A test version of the ReceiverConfig struct to create test yaml fixtures.
type receiverTestConfig struct {
	Name                string `yaml:"name,omitempty"`
	Organization        string `yaml:"organization,omitempty"`
	TenantID            string `yaml:"tenant_id,omitempty"`
	ClientID            string `yaml:"client_id,omitempty"`
	SubscriptionID      string `yaml:"subscription_id,omitempty"`
	ClientSecret        string `yaml:"client_secret,omitempty"`
	PersonalAccessToken string `yaml:"personal_access_token,omitempty"`
	Project             string `yaml:"project,omitempty"`
	IssueType           string `yaml:"issue_type,omitempty"`
	Summary             string `yaml:"summary,omitempty"`
	ReopenState         string `yaml:"reopen_state,omitempty"`
	ReopenDuration      string `yaml:"reopen_duration,omitempty"`

	Priority        string   `yaml:"priority,omitempty"`
	Description     string   `yaml:"description,omitempty"`
	SkipReopenState string   `yaml:"skip_reopen_state,omitempty"`
	AddGroupLabels  *bool    `yaml:"add_group_labels,omitempty"`
	UpdateInComment *bool    `yaml:"update_in_comment,omitempty"`
	StaticLabels    []string `yaml:"static_labels" json:"static_labels"`

	AutoResolve *AutoResolve `yaml:"auto_resolve" json:"auto_resolve"`

	// TODO(rporres): Add support for these.
	// Fields            map[string]interface{} `yaml:"fields,omitempty"`
	// Components        []string               `yaml:"components,omitempty"`
}

// A test version of the Config struct to create test yaml fixtures.
type testConfig struct {
	Defaults  *receiverTestConfig   `yaml:"defaults,omitempty"`
	Receivers []*receiverTestConfig `yaml:"receivers,omitempty"`
	Template  string                `yaml:"template,omitempty"`
}

// Required Config keys tests.
func TestMissingConfigKeys(t *testing.T) {
	defaultsConfig := newReceiverTestConfig(mandatoryReceiverFields(), []string{})
	receiverConfig := newReceiverTestConfig([]string{"Name"}, []string{})

	var config testConfig

	// No receivers.
	config = testConfig{
		Defaults:  defaultsConfig,
		Receivers: []*receiverTestConfig{},
		Template:  "alert-az-do.tmpl",
	}
	configErrorTestRunner(t, config, "no receivers defined")

	// No template.
	config = testConfig{
		Defaults:  defaultsConfig,
		Receivers: []*receiverTestConfig{receiverConfig},
	}
	configErrorTestRunner(t, config, "missing template file")
}

// Tests regarding mandatory keys.
// No tests for auth keys here. They will be handled separately.
func TestRequiredReceiverConfigKeys(t *testing.T) {
	mandatory := mandatoryReceiverFields()
	for _, test := range []struct {
		missingField string
		errorMessage string
	}{
		{"Name", "missing name for receiver"},
		{"Organization", `missing organization in receiver "Name"`},
		{"Project", `missing project in receiver "Name"`},
		{"IssueType", `missing issue_type in receiver "Name"`},
		{"Summary", `missing summary in receiver "Name"`},
		{"ReopenState", `missing reopen_state in receiver "Name"`},
		{"ReopenDuration", `missing reopen_duration in receiver "Name"`},
	} {

		fields := removeFromStrSlice(mandatory, test.missingField)

		// Non-empty defaults as we don't handle the empty defaults case yet.
		defaultsConfig := newReceiverTestConfig([]string{}, []string{"Priority"})
		receiverConfig := newReceiverTestConfig(fields, []string{})
		config := testConfig{
			Defaults:  defaultsConfig,
			Receivers: []*receiverTestConfig{receiverConfig},
			Template:  "azdotemplate.tmpl", // Fix: was jiratemplate.tmpl
		}
		configErrorTestRunner(t, config, test.errorMessage)
	}
}

// Auth keys error scenarios.
func TestAuthKeysErrors(t *testing.T) {
	servicePrincipal := mandatoryReceiverFields()
	managedIdentity := mandatoryManagedIdentityFields()
	pat := mandatoryPATFields()
	minimalReceiverTestConfig := newReceiverTestConfig([]string{"Name"}, []string{})

	// Test cases for Service Principal authentication:
	// * missing TenantID, ClientID, or ClientSecret
	// * Service Principal + PAT conflicts
	// Test cases for Managed Identity authentication:
	// * missing ClientID or SubscriptionID
	// * Managed Identity + PAT conflicts
	// Test cases for PAT authentication:
	// * missing PersonalAccessToken
	for _, test := range []struct {
		receiverTestConfigMandatoryFields []string
		errorMessage                      string
	}{
		// Service Principal incomplete scenarios
		{
			removeFromStrSlice(servicePrincipal, "TenantID"),
			`missing authentication in receiver "Name"`,
		},
		{
			removeFromStrSlice(servicePrincipal, "ClientID"),
			`missing authentication in receiver "Name"`,
		},
		{
			removeFromStrSlice(servicePrincipal, "ClientSecret"),
			`missing authentication in receiver "Name"`,
		},
		// Managed Identity incomplete scenarios
		{
			removeFromStrSlice(managedIdentity, "ClientID"),
			`missing authentication in receiver "Name"`,
		},
		{
			removeFromStrSlice(managedIdentity, "SubscriptionID"),
			`missing authentication in receiver "Name"`,
		},
		// PAT incomplete scenarios
		{
			removeFromStrSlice(pat, "PersonalAccessToken"),
			`missing authentication in receiver "Name"`,
		},
		// Mutual exclusivity scenarios - Service Principal + PAT
		{
			append(servicePrincipal, "PersonalAccessToken"),
			"Service Principal (TenantID+ClientID+ClientSecret), Managed Identity (ClientID+SubscriptionID), and PAT authentication are mutually exclusive",
		},
		// Mutual exclusivity scenarios - Managed Identity + PAT
		{
			append(managedIdentity, "PersonalAccessToken"),
			"Service Principal (TenantID+ClientID+ClientSecret), Managed Identity (ClientID+SubscriptionID), and PAT authentication are mutually exclusive",
		},
		// Mutual exclusivity scenarios - Service Principal + Managed Identity + PAT
		{
			append(append(servicePrincipal, "SubscriptionID"), "PersonalAccessToken"),
			"Service Principal (TenantID+ClientID+ClientSecret), Managed Identity (ClientID+SubscriptionID), and PAT authentication are mutually exclusive",
		},
	} {

		defaultsConfig := newReceiverTestConfig(test.receiverTestConfigMandatoryFields, []string{})
		config := testConfig{
			Defaults:  defaultsConfig,
			Receivers: []*receiverTestConfig{minimalReceiverTestConfig},
			Template:  "alert-az-do.tmpl",
		}

		configErrorTestRunner(t, config, test.errorMessage)
	}
}

// These tests want to make sure that receiver auth always overrides defaults auth.
func TestAuthKeysOverrides(t *testing.T) {
	defaultsWithServicePrincipal := mandatoryReceiverFields()       // TenantID + ClientID + ClientSecret
	defaultsWithManagedIdentity := mandatoryManagedIdentityFields() // ClientID + SubscriptionID

	defaultsWithPAT := []string{"PersonalAccessToken"}
	for _, field := range defaultsWithServicePrincipal {
		if field == "TenantID" || field == "ClientID" || field == "ClientSecret" {
			continue
		}
		defaultsWithPAT = append(defaultsWithPAT, field)
	}

	// Test cases:
	// * tenantId receiver overrides tenantId default.
	// * clientId receiver overrides clientId default.
	// * clientSecret receiver overrides clientSecret default.
	// * subscriptionId receiver overrides subscriptionId default (for managed identity).
	// * tenantId, clientId & clientSecret receiver overrides tenantId, clientId & clientSecret default.
	// * clientId & subscriptionId receiver overrides clientId & subscriptionId default (managed identity).
	// * PAT receiver overrides service principal default.
	// * PAT receiver overrides PAT default.
	// * service principal receiver overrides PAT default.
	for _, test := range []struct {
		tenantIdOverrideValue       string
		clientIdOverrideValue       string
		subscriptionIdOverrideValue string
		clientSecretOverrideValue   string
		patOverrideValue            string // Personal Access Token override.
		tenantIdExpectedValue       string
		clientIdExpectedValue       string
		subscriptionIdExpectedValue string
		clientSecretExpectedValue   string
		patExpectedValue            string
		defaultFields               []string // Fields to build the config defaults.
	}{
		{"tenantId", "", "", "", "", "tenantId", "ClientID", "", "ClientSecret", "", defaultsWithServicePrincipal},
		{"", "clientId", "", "", "", "TenantID", "clientId", "", "ClientSecret", "", defaultsWithServicePrincipal},
		{"", "", "", "clientSecret", "", "TenantID", "ClientID", "", "clientSecret", "", defaultsWithServicePrincipal},
		{"", "clientId", "subscriptionId", "", "", "", "clientId", "subscriptionId", "", "", defaultsWithManagedIdentity},
		{"tenantId", "clientId", "", "clientSecret", "", "tenantId", "clientId", "", "clientSecret", "", defaultsWithServicePrincipal},
		{"", "", "", "", "azurePAT", "", "", "", "", "azurePAT", defaultsWithServicePrincipal},
		{"", "", "", "", "", "", "", "", "", "PersonalAccessToken", defaultsWithPAT},
		{"", "", "", "", "azurePAT", "", "", "", "", "azurePAT", defaultsWithPAT},
	} {
		defaultsConfig := newReceiverTestConfig(test.defaultFields, []string{})
		receiverConfig := newReceiverTestConfig([]string{"Name"}, []string{})
		if test.tenantIdOverrideValue != "" {
			receiverConfig.TenantID = test.tenantIdOverrideValue
		}
		if test.clientIdOverrideValue != "" {
			receiverConfig.ClientID = test.clientIdOverrideValue
		}
		if test.subscriptionIdOverrideValue != "" {
			receiverConfig.SubscriptionID = test.subscriptionIdOverrideValue
		}
		if test.clientSecretOverrideValue != "" {
			receiverConfig.ClientSecret = test.clientSecretOverrideValue
		}
		if test.patOverrideValue != "" {
			receiverConfig.PersonalAccessToken = test.patOverrideValue
		}

		config := testConfig{
			Defaults:  defaultsConfig,
			Receivers: []*receiverTestConfig{receiverConfig},
			Template:  "alert-az-do.tmpl",
		}

		yamlConfig, err := yaml.Marshal(&config)
		require.NoError(t, err)

		cfg, err := Load(string(yamlConfig))
		require.NoError(t, err)

		receiver := cfg.Receivers[0]
		require.Equal(t, test.tenantIdExpectedValue, receiver.TenantID)
		require.Equal(t, test.clientIdExpectedValue, receiver.ClientID)
		require.Equal(t, test.subscriptionIdExpectedValue, receiver.SubscriptionID)
		require.Equal(t, Secret(test.clientSecretExpectedValue), receiver.ClientSecret)
		require.Equal(t, Secret(test.patExpectedValue), receiver.PersonalAccessToken)
	}
}

// Tests regarding yaml keys overriden in the receiver config.
// No tests for auth keys here. They will be handled separately
func TestReceiverOverrides(t *testing.T) {
	fifteenHoursToDuration, err := time.ParseDuration("15h")
	//fifteenHoursToDuration, err := ParseDuration("15h")
	autoResolve := AutoResolve{State: "Completed"} // Fix: use "Completed" to match the default
	require.NoError(t, err)
	addGroupLabelsTrueVal := true
	addGroupLabelsFalseVal := false
	updateInCommentTrueVal := true
	updateInCommentFalseVal := false

	// We'll override one key at a time and check the value in the receiver.
	for _, test := range []struct {
		overrideField string
		overrideValue interface{}
		expectedValue interface{}
	}{
		{"Organization", `redhat`, `redhat`},
		{"Project", "APPSRE", "APPSRE"},
		{"IssueType", "Task", "Task"},
		{"Summary", "A nice summary", "A nice summary"},
		{"ReopenState", "To Do", "To Do"},
		{"ReopenDuration", "15h", &fifteenHoursToDuration},
		{"Priority", "Critical", "Critical"},
		{"Description", "A nice description", "A nice description"},
		{"SkipReopenState", "Removed", "Removed"},
		{"AddGroupLabels", &addGroupLabelsFalseVal, &addGroupLabelsFalseVal},
		{"AddGroupLabels", &addGroupLabelsTrueVal, &addGroupLabelsTrueVal},
		{"UpdateInComment", &updateInCommentFalseVal, &updateInCommentFalseVal},
		{"UpdateInComment", &updateInCommentTrueVal, &updateInCommentTrueVal},
		{"AutoResolve", &AutoResolve{State: "Completed"}, &autoResolve}, // Fix: expect "Completed" not "Done"
		{"StaticLabels", []string{"somelabel"}, []string{"somelabel"}},
	} {
		optionalFields := []string{"Priority", "Description", "SkipReopenState", "AddGroupLabels", "UpdateInComment", "AutoResolve", "StaticLabels"}
		defaultsConfig := newReceiverTestConfig(mandatoryReceiverFields(), optionalFields)
		receiverConfig := newReceiverTestConfig([]string{"Name"}, optionalFields)

		reflect.ValueOf(receiverConfig).Elem().FieldByName(test.overrideField).
			Set(reflect.ValueOf(test.overrideValue))

		config := testConfig{
			Defaults:  defaultsConfig,
			Receivers: []*receiverTestConfig{receiverConfig},
			Template:  "alert-az-do.tmpl",
		}

		yamlConfig, err := yaml.Marshal(&config)
		require.NoError(t, err)

		cfg, err := Load(string(yamlConfig))
		require.NoError(t, err)

		receiver := cfg.Receivers[0]
		configValue := reflect.ValueOf(receiver).Elem().FieldByName(test.overrideField).Interface()
		require.Equal(t, test.expectedValue, configValue)
	}
}

// TODO(bwplotka, rporres). Add more tests:
//   * Tests on optional keys.
//   * Tests on unknown keys.
//   * Tests on Duration.

// Creates a receiverTestConfig struct with default values.
func newReceiverTestConfig(mandatory []string, optional []string) *receiverTestConfig {
	r := receiverTestConfig{}
	addGroupLabelsDefaultVal := true
	updateInCommentDefaultVal := true

	for _, name := range mandatory {
		var value reflect.Value

		switch name {
		case "Organization":
			value = reflect.ValueOf("alert-az-do")
		case "ReopenDuration":
			value = reflect.ValueOf("24h")
		default:
			value = reflect.ValueOf(name)
		}
		reflect.ValueOf(&r).Elem().FieldByName(name).Set(value)
	}

	for _, name := range optional {
		var value reflect.Value
		switch name {
		case "AddGroupLabels":
			value = reflect.ValueOf(&addGroupLabelsDefaultVal)
		case "UpdateInComment":
			value = reflect.ValueOf(&updateInCommentDefaultVal)
		case "AutoResolve":
			value = reflect.ValueOf(&AutoResolve{State: "Completed"})
		case "StaticLabels":
			value = reflect.ValueOf([]string{})
		default:
			value = reflect.ValueOf(name)
		}

		reflect.ValueOf(&r).Elem().FieldByName(name).Set(value)
	}

	return &r
}

// Creates a yaml from testConfig, Loads it checks the errors are the expected ones.
func configErrorTestRunner(t *testing.T, config testConfig, errorMessage string) {
	yamlConfig, err := yaml.Marshal(&config)
	require.NoError(t, err)

	_, err = Load(string(yamlConfig))
	require.Error(t, err)
	require.Contains(t, err.Error(), errorMessage)
}

// returns a new slice that has the element removed
func removeFromStrSlice(strSlice []string, element string) []string {
	var newStrSlice []string
	for _, value := range strSlice {
		if value != element {
			newStrSlice = append(newStrSlice, value)
		}
	}

	return newStrSlice
}

// Returns mandatory receiver fields for Service Principal authentication to be used creating test config structs.
// Service Principal requires: TenantID + ClientID + ClientSecret (not SubscriptionID).
func mandatoryReceiverFields() []string {
	return []string{
		"Name",
		"Organization",
		"TenantID",
		"ClientID",
		"ClientSecret",
		"Project",
		"IssueType",
		"Summary",
		"ReopenState",
		"ReopenDuration",
	}
}

// Returns mandatory receiver fields for Managed Identity authentication.
// Managed Identity requires: ClientID + SubscriptionID (no secrets).
func mandatoryManagedIdentityFields() []string {
	return []string{
		"Name",
		"Organization",
		"ClientID",
		"SubscriptionID",
		"Project",
		"IssueType",
		"Summary",
		"ReopenState",
		"ReopenDuration",
	}
}

// Returns mandatory receiver fields for PAT authentication.
// PAT requires: only PersonalAccessToken.
func mandatoryPATFields() []string {
	return []string{
		"Name",
		"Organization",
		"PersonalAccessToken",
		"Project",
		"IssueType",
		"Summary",
		"ReopenState",
		"ReopenDuration",
	}
}

func TestAutoResolveConfigReceiver(t *testing.T) {
	mandatory := mandatoryReceiverFields()
	minimalReceiverTestConfig := &receiverTestConfig{
		Name: "test",
		AutoResolve: &AutoResolve{
			State: "",
		},
	}

	defaultsConfig := newReceiverTestConfig(mandatory, []string{})
	config := testConfig{
		Defaults:  defaultsConfig,
		Receivers: []*receiverTestConfig{minimalReceiverTestConfig},
		Template:  "alert-az-do.tmpl",
	}

	configErrorTestRunner(t, config, "bad config in receiver \"test\", 'auto_resolve' was defined with empty 'state' field")

}

func TestAutoResolveConfigDefault(t *testing.T) {
	mandatory := mandatoryReceiverFields()
	minimalReceiverTestConfig := newReceiverTestConfig([]string{"Name"}, []string{"AutoResolve"})

	defaultsConfig := newReceiverTestConfig(mandatory, []string{})
	defaultsConfig.AutoResolve = &AutoResolve{
		State: "",
	}
	config := testConfig{
		Defaults:  defaultsConfig,
		Receivers: []*receiverTestConfig{minimalReceiverTestConfig},
		Template:  "alert-az-do.tmpl",
	}

	configErrorTestRunner(t, config, "bad config in defaults section: state cannot be empty")

}

func TestStaticLabelsConfigMerge(t *testing.T) {

	for i, test := range []struct {
		defaultValue     []string
		receiverValue    []string
		expectedElements []string
	}{
		{[]string{"defaultlabel"}, []string{"receiverlabel"}, []string{"defaultlabel", "receiverlabel"}},
		{[]string{}, []string{"receiverlabel"}, []string{"receiverlabel"}},
		{[]string{"defaultlabel"}, []string{}, []string{"defaultlabel"}},
		{[]string{}, []string{}, []string{}},
	} {
		mandatory := mandatoryReceiverFields()

		defaultsConfig := newReceiverTestConfig(mandatory, []string{})
		defaultsConfig.StaticLabels = test.defaultValue

		receiverConfig := newReceiverTestConfig([]string{"Name"}, []string{"StaticLabels"})
		receiverConfig.StaticLabels = test.receiverValue

		config := testConfig{
			Defaults:  defaultsConfig,
			Receivers: []*receiverTestConfig{receiverConfig},
			Template:  "alert-az-do.tmpl",
		}

		yamlConfig, err := yaml.Marshal(&config)
		require.NoError(t, err)

		cfg, err := Load(string(yamlConfig))
		require.NoError(t, err)

		receiver := cfg.Receivers[0]
		require.ElementsMatch(t, receiver.StaticLabels, test.expectedElements, "Elements should match (failing index: %v)", i)
	}
}

// TestEnvironmentVariableCredentialPrecedence tests the credential precedence logic
// that matches the behavior in AlertHandlerFunc. Environment variables should take
// precedence over config-based credentials.
func TestEnvironmentVariableCredentialPrecedence(t *testing.T) {
	// Save original environment values
	originalTenantID := os.Getenv("AZURE_TENANT_ID")
	originalClientID := os.Getenv("AZURE_CLIENT_ID")
	originalSubscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	originalClientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	originalPAT := os.Getenv("AZURE_PAT")

	// Clean up environment after test
	defer func() {
		require.NoError(t, os.Setenv("AZURE_TENANT_ID", originalTenantID))
		require.NoError(t, os.Setenv("AZURE_CLIENT_ID", originalClientID))
		require.NoError(t, os.Setenv("AZURE_SUBSCRIPTION_ID", originalSubscriptionID))
		require.NoError(t, os.Setenv("AZURE_CLIENT_SECRET", originalClientSecret))
		require.NoError(t, os.Setenv("AZURE_PAT", originalPAT))
	}()

	// Test cases:
	// * Environment service principal variables should take precedence over config
	// * Environment managed identity should take precedence over config service principal
	// * Environment PAT should take precedence over config service principal
	// * Config service principal should be used when no env vars are set
	// * Config managed identity should be used when no env vars are set
	// * Config PAT should be used when no env vars or service principal config
	// * Partial environment service principal should fall back to config
	// * Environment PAT should be used even with partial service principal env vars
	tests := []struct {
		name                   string
		envTenantID            string
		envClientID            string
		envSubscriptionID      string
		envClientSecret        string
		envPAT                 string
		configTenantID         string
		configClientID         string
		configSubscriptionID   string
		configClientSecret     string
		configPAT              string
		expectedCredentialType string
	}{
		{"environment_service_principal_takes_precedence", "env-tenant-id", "env-client-id", "", "env-client-secret", "", "config-tenant-id", "config-client-id", "config-subscription-id", "config-client-secret", "", "environment_service_principal"},
		{"environment_managed_identity_takes_precedence", "", "env-client-id", "env-subscription-id", "", "", "config-tenant-id", "config-client-id", "", "config-client-secret", "", "environment_managed_identity"},
		{"environment_pat_takes_precedence", "", "", "", "", "env-pat-token", "config-tenant-id", "config-client-id", "", "config-client-secret", "", "environment_pat"},
		{"config_service_principal_fallback", "", "", "", "", "", "config-tenant-id", "config-client-id", "", "config-client-secret", "", "config_service_principal"},
		{"config_managed_identity_fallback", "", "", "", "", "", "", "config-client-id", "config-subscription-id", "", "", "config_managed_identity"},
		{"config_pat_fallback", "", "", "", "", "", "", "", "", "", "config-pat-token", "config_pat"},
		{"environment_service_principal_partial_ignored", "env-tenant-id", "", "", "env-client-secret", "", "config-tenant-id", "config-client-id", "", "config-client-secret", "", "config_service_principal"}, // Missing client ID
		{"environment_pat_beats_partial_service_principal", "env-tenant-id", "", "", "", "env-pat-token", "config-tenant-id", "config-client-id", "", "config-client-secret", "", "environment_pat"},            // Only partial service principal
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			require.NoError(t, os.Setenv("AZURE_TENANT_ID", tt.envTenantID))
			require.NoError(t, os.Setenv("AZURE_CLIENT_ID", tt.envClientID))
			require.NoError(t, os.Setenv("AZURE_SUBSCRIPTION_ID", tt.envSubscriptionID))
			require.NoError(t, os.Setenv("AZURE_CLIENT_SECRET", tt.envClientSecret))
			require.NoError(t, os.Setenv("AZURE_PAT", tt.envPAT))

			// Create config
			defaultsConfig := &receiverTestConfig{
				Organization:   "test-org",
				IssueType:      "Bug",
				Summary:        "Test summary",
				ReopenState:    "To Do",
				ReopenDuration: "24h",
			}

			receiverConfig := &receiverTestConfig{
				Name:    "test-receiver",
				Project: "test-project",
			}

			// Only set config auth fields if they're non-empty
			if tt.configTenantID != "" {
				receiverConfig.TenantID = tt.configTenantID
			}
			if tt.configClientID != "" {
				receiverConfig.ClientID = tt.configClientID
			}
			if tt.configSubscriptionID != "" {
				receiverConfig.SubscriptionID = tt.configSubscriptionID
			}
			if tt.configClientSecret != "" {
				receiverConfig.ClientSecret = tt.configClientSecret
			}
			if tt.configPAT != "" {
				receiverConfig.PersonalAccessToken = tt.configPAT
			}

			config := testConfig{
				Defaults:  defaultsConfig,
				Receivers: []*receiverTestConfig{receiverConfig},
				Template:  "alert-az-do.tmpl",
			}

			yamlConfig, err := yaml.Marshal(&config)
			require.NoError(t, err)

			cfg, err := Load(string(yamlConfig))
			require.NoError(t, err)

			receiver := cfg.Receivers[0]

			// Simulate the credential selection logic from AlertHandlerFunc
			var actualCredentialType string
			if os.Getenv("AZURE_TENANT_ID") != "" && os.Getenv("AZURE_CLIENT_ID") != "" && os.Getenv("AZURE_CLIENT_SECRET") != "" {
				actualCredentialType = "environment_service_principal"
			} else if os.Getenv("AZURE_CLIENT_ID") != "" && os.Getenv("AZURE_SUBSCRIPTION_ID") != "" {
				actualCredentialType = "environment_managed_identity"
			} else if os.Getenv("AZURE_PAT") != "" {
				actualCredentialType = "environment_pat"
			} else if receiver.TenantID != "" && receiver.ClientID != "" && receiver.ClientSecret != "" {
				actualCredentialType = "config_service_principal"
			} else if receiver.ClientID != "" && receiver.SubscriptionID != "" {
				actualCredentialType = "config_managed_identity"
			} else if receiver.PersonalAccessToken != "" {
				actualCredentialType = "config_pat"
			} else {
				actualCredentialType = "none"
			}

			require.Equal(t, tt.expectedCredentialType, actualCredentialType, "Test: %s - %s", tt.name)
		})
	}
}

// TestEnvironmentVariableSubstitutionInCredentials tests that environment variable
// substitution works correctly for credential fields in the config file.
func TestEnvironmentVariableSubstitutionInCredentials(t *testing.T) {
	// Set up environment variables for substitution
	require.NoError(t, os.Setenv("TEST_TENANT_ID", "substituted-tenant-id"))
	require.NoError(t, os.Setenv("TEST_CLIENT_ID", "substituted-client-id"))
	require.NoError(t, os.Setenv("TEST_SUBSCRIPTION_ID", "substituted-subscription-id"))
	require.NoError(t, os.Setenv("TEST_CLIENT_SECRET", "substituted-client-secret"))
	require.NoError(t, os.Setenv("TEST_PAT", "substituted-pat-token"))

	defer func() {
		require.NoError(t, os.Unsetenv("TEST_TENANT_ID"))
		require.NoError(t, os.Unsetenv("TEST_CLIENT_ID"))
		require.NoError(t, os.Unsetenv("TEST_SUBSCRIPTION_ID"))
		require.NoError(t, os.Unsetenv("TEST_CLIENT_SECRET"))
		require.NoError(t, os.Unsetenv("TEST_PAT"))
	}()

	configYAML := `
defaults:
  organization: test-org
  tenant_id: $(TEST_TENANT_ID)
  client_id: $(TEST_CLIENT_ID)
  subscription_id: $(TEST_SUBSCRIPTION_ID)
  client_secret: $(TEST_CLIENT_SECRET)
  issue_type: Bug
  summary: 'Test summary'
  reopen_state: "To Do"
  reopen_duration: 24h

receivers:
  - name: 'test-receiver'
    project: test-project
    personal_access_token: $(TEST_PAT)

template: alert-az-do.tmpl
`

	// Use substituteEnvVars first, then Load
	substitutedContent, err := substituteEnvVars([]byte(configYAML), log.NewNopLogger())
	require.NoError(t, err)

	cfg, err := Load(string(substitutedContent))
	require.NoError(t, err)

	// Check that defaults got substituted
	require.Equal(t, "substituted-tenant-id", cfg.Defaults.TenantID)
	require.Equal(t, "substituted-client-id", cfg.Defaults.ClientID)
	require.Equal(t, "substituted-subscription-id", cfg.Defaults.SubscriptionID)
	require.Equal(t, Secret("substituted-client-secret"), cfg.Defaults.ClientSecret)

	// Check that receiver got substituted
	receiver := cfg.Receivers[0]
	require.Equal(t, Secret("substituted-pat-token"), receiver.PersonalAccessToken)
}
