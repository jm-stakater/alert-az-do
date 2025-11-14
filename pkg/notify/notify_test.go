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

package notify

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/jm-stakater/alert-az-do/pkg/alertmanager"
	"github.com/jm-stakater/alert-az-do/pkg/config"
	"github.com/jm-stakater/alert-az-do/pkg/template"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/require"
)

// Keep only helper functions and test functions from the original file

func containsSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func testReceiverConfig1() *config.ReceiverConfig {
	return &config.ReceiverConfig{
		Project:     "TestProject",
		IssueType:   "Bug",
		Summary:     `[{{ .Status | toUpper }}{{ if eq .Status "firing" }}:{{ .Alerts.Firing | len }}{{ end }}] {{ .GroupLabels.SortedPairs.Values | join " " }}`,
		Description: `Alert Description: {{ .CommonAnnotations.description }}`,
		Fields:      map[string]interface{}{},
	}
}

func testReceiverConfigWithFields() *config.ReceiverConfig {
	return &config.ReceiverConfig{
		Project:     "TestProject",
		IssueType:   "Task",
		Summary:     `[{{ .Status | toUpper }}] Alert Summary`,
		Description: `Alert fired with {{ .Alerts.Firing | len }} alerts`,
		Fields: map[string]interface{}{
			"System.Priority": "High",
			"Custom.Field":    "{{ .CommonLabels.severity }}",
		},
	}
}

func TestReceiver_Notify_CreateWorkItem(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	mockClient := newMockWorkItemTrackingClient()
	tmpl := template.SimpleTemplate()
	config := testReceiverConfig1()

	receiver := &Receiver{
		logger: logger,
		client: mockClient,
		conf:   config,
		tmpl:   tmpl,
	}

	data := &alertmanager.Data{
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertFiring,
				Fingerprint: "test-fingerprint-123",
			},
		},
		Status: alertmanager.AlertFiring,
		GroupLabels: alertmanager.KV{
			"alertname": "TestAlert",
			"severity":  "critical",
		},
		CommonAnnotations: alertmanager.KV{
			"description": "Test alert description",
		},
	}

	ctx := context.Background()
	err := receiver.Notify(ctx, data)

	require.NoError(t, err)
	require.Len(t, mockClient.createCalls, 1)
	require.Len(t, mockClient.updateCalls, 0)

	createCall := mockClient.createCalls[0]
	require.Equal(t, "TestProject", *createCall.args.Project)
	require.Equal(t, "Bug", *createCall.args.Type)

	// Check that the document contains the expected operations
	var titleOp, descriptionOp, tagsOp *webapi.JsonPatchOperation
	for _, op := range *createCall.args.Document {
		if op.Path != nil {
			switch *op.Path {
			case "/fields/System.Title":
				titleOp = &op
			case "/fields/System.Description":
				descriptionOp = &op
			case "/fields/System.Tags":
				tagsOp = &op
			}
		}
	}

	require.NotNil(t, titleOp)
	require.Contains(t, titleOp.Value, "[FIRING:1]")
	require.NotNil(t, descriptionOp)
	require.NotNil(t, tagsOp)
	require.Contains(t, tagsOp.Value, "Fingerprint:test-fingerprint-123")
}

func TestReceiver_Notify_UpdateExistingWorkItem(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	mockClient := newMockWorkItemTrackingClient()
	tmpl := template.SimpleTemplate()
	config := testReceiverConfig1()

	receiver := &Receiver{
		logger: logger,
		client: mockClient,
		conf:   config,
		tmpl:   tmpl,
	}

	// First create a work item
	fingerprint := "test-fingerprint-123"
	existingWorkItem := &workitemtracking.WorkItem{
		Id: func() *int { i := 1; return &i }(),
		Fields: &map[string]interface{}{
			"System.Title":       "[FIRING:1] Test Alert",
			"System.Description": "Original description",
			"System.Tags":        fmt.Sprintf("Fingerprint:%s", fingerprint),
		},
	}
	mockClient.workItems[1] = existingWorkItem
	mockClient.workItemsByTag[fmt.Sprintf("Fingerprint:%s", fingerprint)] = []*workitemtracking.WorkItem{existingWorkItem}

	data := &alertmanager.Data{
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertFiring,
				Fingerprint: fingerprint,
			},
			alertmanager.Alert{
				Status:      alertmanager.AlertFiring,
				Fingerprint: fingerprint,
			},
		},
		Status: alertmanager.AlertFiring,
		GroupLabels: alertmanager.KV{
			"alertname": "TestAlert",
		},
		CommonAnnotations: alertmanager.KV{
			"description": "Updated alert description",
		},
	}

	ctx := context.Background()
	err := receiver.Notify(ctx, data)

	require.NoError(t, err)
	require.Len(t, mockClient.createCalls, 0)
	require.Len(t, mockClient.updateCalls, 1)

	updateCall := mockClient.updateCalls[0]
	require.Equal(t, 1, *updateCall.args.Id)
}

func TestReceiver_Notify_ResolveWorkItem(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	mockClient := newMockWorkItemTrackingClient()
	tmpl := template.SimpleTemplate()
	config := testReceiverConfig1()

	receiver := &Receiver{
		logger: logger,
		client: mockClient,
		conf:   config,
		tmpl:   tmpl,
	}

	// Create an existing work item
	fingerprint := "test-fingerprint-123"
	existingWorkItem := &workitemtracking.WorkItem{
		Id: func() *int { i := 1; return &i }(),
		Fields: &map[string]interface{}{
			"System.Title":       "[FIRING:1] Test Alert",
			"System.Description": "Alert description",
			"System.Tags":        fmt.Sprintf("Fingerprint:%s", fingerprint),
			"System.State":       "Active",
		},
	}
	mockClient.workItems[1] = existingWorkItem
	mockClient.workItemsByTag[fmt.Sprintf("Fingerprint:%s", fingerprint)] = []*workitemtracking.WorkItem{existingWorkItem}

	data := &alertmanager.Data{
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertResolved,
				Fingerprint: fingerprint,
			},
		},
		Status: alertmanager.AlertResolved,
		GroupLabels: alertmanager.KV{
			"alertname": "TestAlert",
		},
	}

	ctx := context.Background()
	err := receiver.Notify(ctx, data)

	require.NoError(t, err)
	require.Len(t, mockClient.createCalls, 0)
	require.Len(t, mockClient.updateCalls, 1)

	updateCall := mockClient.updateCalls[0]
	require.Equal(t, 1, *updateCall.args.Id)

	// Verify the updated work item has resolved status in title
	updatedWorkItem := mockClient.workItems[1]
	title := (*updatedWorkItem.Fields)["System.Title"].(string)
	require.Contains(t, title, "[RESOLVED]")
}

func TestReceiver_Notify_WithCustomFields(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	mockClient := newMockWorkItemTrackingClient()
	tmpl := template.SimpleTemplate()
	config := testReceiverConfigWithFields()

	receiver := &Receiver{
		logger: logger,
		client: mockClient,
		conf:   config,
		tmpl:   tmpl,
	}

	data := &alertmanager.Data{
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertFiring,
				Fingerprint: "test-fingerprint-456",
			},
		},
		Status: alertmanager.AlertFiring,
		GroupLabels: alertmanager.KV{
			"alertname": "TestAlert",
		},
		CommonLabels: alertmanager.KV{
			"severity": "critical",
		},
	}

	ctx := context.Background()
	err := receiver.Notify(ctx, data)

	require.NoError(t, err)
	require.Len(t, mockClient.createCalls, 1)

	createCall := mockClient.createCalls[0]
	require.Equal(t, "TestProject", *createCall.args.Project)
	require.Equal(t, "Task", *createCall.args.Type)

	// Check for custom fields in the document
	var priorityOp, customFieldOp *webapi.JsonPatchOperation
	for _, op := range *createCall.args.Document {
		if op.Path != nil {
			switch *op.Path {
			case "/fields/System.Priority":
				priorityOp = &op
			case "/fields/Custom.Field":
				customFieldOp = &op
			}
		}
	}

	require.NotNil(t, priorityOp)
	require.Equal(t, "High", priorityOp.Value)
	require.NotNil(t, customFieldOp)
	require.Equal(t, "critical", customFieldOp.Value)
}

func TestReceiver_FindWorkItem_NotFound(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	mockClient := newMockWorkItemTrackingClient()
	tmpl := template.SimpleTemplate()
	config := testReceiverConfig1()

	receiver := &Receiver{
		logger: logger,
		client: mockClient,
		conf:   config,
		tmpl:   tmpl,
	}

	data := &alertmanager.Data{
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertFiring,
				Fingerprint: "nonexistent-fingerprint",
			},
		},
	}

	ctx := context.Background()
	workItem, err := receiver.findWorkItem(ctx, data, "TestProject")

	require.NoError(t, err)
	require.Nil(t, workItem)
	require.Len(t, mockClient.queryCalls, 1)
	require.Contains(t, mockClient.queryCalls[0], "Fingerprint:nonexistent-fingerprint")
}

func TestReceiver_FindWorkItem_Found(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	mockClient := newMockWorkItemTrackingClient()
	tmpl := template.SimpleTemplate()
	config := testReceiverConfig1()

	receiver := &Receiver{
		logger: logger,
		client: mockClient,
		conf:   config,
		tmpl:   tmpl,
	}

	// Create an existing work item
	fingerprint := "existing-fingerprint"
	existingWorkItem := &workitemtracking.WorkItem{
		Id: func() *int { i := 42; return &i }(),
		Fields: &map[string]interface{}{
			"System.Title": "Existing Alert",
			"System.Tags":  fmt.Sprintf("Fingerprint:%s", fingerprint),
		},
	}
	mockClient.workItems[42] = existingWorkItem
	mockClient.workItemsByTag[fmt.Sprintf("Fingerprint:%s", fingerprint)] = []*workitemtracking.WorkItem{existingWorkItem}

	data := &alertmanager.Data{
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertFiring,
				Fingerprint: fingerprint,
			},
		},
	}

	ctx := context.Background()
	workItem, err := receiver.findWorkItem(ctx, data, "TestProject")

	require.NoError(t, err)
	require.NotNil(t, workItem)
	require.Equal(t, 42, *workItem.Id)
	require.Len(t, mockClient.queryCalls, 1)
	require.Contains(t, mockClient.queryCalls[0], fmt.Sprintf("Fingerprint:%s", fingerprint))
}

func TestReceiver_GenerateWorkItemDocument(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	tmpl := template.SimpleTemplate()
	config := testReceiverConfigWithFields()

	receiver := &Receiver{
		logger: logger,
		conf:   config,
		tmpl:   tmpl,
	}

	data := &alertmanager.Data{
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertFiring,
				Fingerprint: "test-fingerprint",
			},
		},
		Status: alertmanager.AlertFiring,
		GroupLabels: alertmanager.KV{
			"alertname": "TestAlert",
		},
		CommonLabels: alertmanager.KV{
			"severity": "critical",
		},
	}

	document, err := receiver.generateWorkItemDocument(data, true)

	require.NoError(t, err)
	require.NotEmpty(t, document)

	// Check that required operations are present
	operationPaths := make(map[string]interface{})
	for _, op := range document {
		if op.Path != nil {
			operationPaths[*op.Path] = op.Value
		}
	}

	require.Contains(t, operationPaths, "/fields/System.Title")
	require.Contains(t, operationPaths, "/fields/System.Description")
	require.Contains(t, operationPaths, "/fields/System.Tags")
	require.Contains(t, operationPaths, "/fields/System.Priority")
	require.Contains(t, operationPaths, "/fields/Custom.Field")

	// Verify fingerprint is in tags
	tags := operationPaths["/fields/System.Tags"].(string)
	require.Contains(t, tags, "Fingerprint:test-fingerprint")

	// Verify custom field templating worked
	customField := operationPaths["/fields/Custom.Field"].(string)
	require.Equal(t, "critical", customField)
}

func TestReceiver_GenerateWorkItemDocument_WithFieldConstants(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	tmpl := template.SimpleTemplate()

	// Create a config that uses both standard and custom fields
	config := &config.ReceiverConfig{
		Project:     "TestProject",
		IssueType:   "Bug",
		Summary:     `[{{ .Status | toUpper }}] {{ .GroupLabels.alertname }}`,
		Description: `Alert: {{ .GroupLabels.alertname }}\nSeverity: {{ .CommonLabels.severity }}`,
		Priority:    "1", // Set priority at the config level, not in Fields
		Fields: map[string]interface{}{
			"System.Reason":            "{{ .GroupLabels.reason }}",
			"Microsoft.VSTS.TCM.Steps": "Investigation steps here",
			"Custom.Environment":       "{{ .CommonLabels.environment }}",
			"Custom.Unknown.Field":     "should be handled gracefully", // This won't have a constant
		},
	}

	receiver := &Receiver{
		logger: logger,
		conf:   config,
		tmpl:   tmpl,
	}

	data := &alertmanager.Data{
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertFiring,
				Fingerprint: "enhanced-test-fingerprint",
			},
		},
		Status: alertmanager.AlertFiring,
		GroupLabels: alertmanager.KV{
			"alertname": "DatabaseConnectivity",
			"reason":    "Connection timeout",
		},
		CommonLabels: alertmanager.KV{
			"severity":    "warning",
			"environment": "production",
		},
	}

	document, err := receiver.generateWorkItemDocument(data, true)

	require.NoError(t, err)
	require.NotEmpty(t, document)

	// Collect all operation paths and values
	operationPaths := make(map[string]interface{})
	for _, op := range document {
		if op.Path != nil {
			operationPaths[*op.Path] = op.Value
		}
	}

	// Verify standard field constants are working
	require.Contains(t, operationPaths, WorkItemFieldTitle.FieldPath())
	require.Contains(t, operationPaths, WorkItemFieldDescription.FieldPath())
	require.Contains(t, operationPaths, WorkItemFieldPriority.FieldPath())
	require.Contains(t, operationPaths, WorkItemFieldReason.FieldPath())
	require.Contains(t, operationPaths, WorkItemFieldTags.FieldPath())

	// Verify Microsoft VSTS field constants
	require.Contains(t, operationPaths, WorkItemFieldSteps.FieldPath())

	// Verify custom fields (both known and unknown)
	require.Contains(t, operationPaths, "/fields/Custom.Environment")
	require.Contains(t, operationPaths, "/fields/Custom.Unknown.Field")

	// Verify field values are correctly templated
	title := operationPaths[WorkItemFieldTitle.FieldPath()].(string)
	require.Equal(t, "[FIRING] DatabaseConnectivity", title)

	description := operationPaths[WorkItemFieldDescription.FieldPath()].(string)
	require.Contains(t, description, "DatabaseConnectivity")
	require.Contains(t, description, "warning")

	priority := operationPaths[WorkItemFieldPriority.FieldPath()].(string)
	require.Equal(t, "1", priority)

	reason := operationPaths[WorkItemFieldReason.FieldPath()].(string)
	require.Equal(t, "Connection timeout", reason)

	tcmSteps := operationPaths[WorkItemFieldSteps.FieldPath()].(string)
	require.Equal(t, "Investigation steps here", tcmSteps)

	environment := operationPaths["/fields/Custom.Environment"].(string)
	require.Equal(t, "production", environment)

	unknownField := operationPaths["/fields/Custom.Unknown.Field"].(string)
	require.Equal(t, "should be handled gracefully", unknownField)

	// Verify fingerprint is properly added to tags
	tags := operationPaths[WorkItemFieldTags.FieldPath()].(string)
	require.Contains(t, tags, "Fingerprint:enhanced-test-fingerprint")
}

func TestReceiver_GenerateWorkItemDocument_FieldConstantValidation(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	tmpl := template.SimpleTemplate()

	// Test all major field groups to ensure constants work correctly
	config := &config.ReceiverConfig{
		Project:     "TestProject",
		IssueType:   "Task",
		Summary:     `Test Summary`,
		Description: `Test Description`,
		Fields: map[string]interface{}{
			// System fields
			"System.State":           "New",
			"System.AssignedTo":      "test@example.com",
			"System.AreaPath":        "TestProject\\Area1",
			"System.IterationPath":   "TestProject\\Sprint1",
			"System.BoardColumn":     "To Do",
			"System.BoardColumnDone": "False",

			// Microsoft VSTS Common fields
			"Microsoft.VSTS.Common.Priority":      "2",
			"Microsoft.VSTS.Common.Severity":      "3 - Medium",
			"Microsoft.VSTS.Common.ValueArea":     "Business",
			"Microsoft.VSTS.Common.Risk":          "Low",
			"Microsoft.VSTS.Common.BusinessValue": "100",

			// Microsoft VSTS Scheduling fields
			"Microsoft.VSTS.Scheduling.Effort":           "5",
			"Microsoft.VSTS.Scheduling.OriginalEstimate": "8",
			"Microsoft.VSTS.Scheduling.RemainingWork":    "6",
			"Microsoft.VSTS.Scheduling.CompletedWork":    "2",
		},
	}

	receiver := &Receiver{
		logger: logger,
		conf:   config,
		tmpl:   tmpl,
	}

	data := &alertmanager.Data{
		Status: alertmanager.AlertFiring,
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertFiring,
				Fingerprint: "validation-test",
			},
		},
	}

	document, err := receiver.generateWorkItemDocument(data, false)

	require.NoError(t, err)
	require.NotEmpty(t, document)

	// Collect operation paths
	operationPaths := make(map[string]interface{})
	for _, op := range document {
		if op.Path != nil {
			operationPaths[*op.Path] = op.Value
		}
	}

	// Test System field constants
	expectedSystemFields := []AzureWorkItemField{
		WorkItemFieldState, WorkItemFieldAssignedTo, WorkItemFieldAreaPath, WorkItemFieldIterationPath,
		WorkItemFieldBoardColumn, WorkItemFieldBoardColumnDone,
	}

	for _, field := range expectedSystemFields {
		require.Contains(t, operationPaths, field.FieldPath(), "Field %s should be present", field)
	}

	// Test Microsoft VSTS Common field constants
	expectedVSTSCommonFields := []AzureWorkItemField{
		WorkItemFieldPriority, WorkItemFieldSeverity,
		WorkItemFieldValueArea, WorkItemFieldRisk,
		WorkItemFieldBusinessValue,
	}

	for _, field := range expectedVSTSCommonFields {
		require.Contains(t, operationPaths, field.FieldPath(), "Field %s should be present", field)
	}

	// Test Microsoft VSTS Scheduling field constants
	expectedVSTSSchedulingFields := []AzureWorkItemField{
		WorkItemFieldEffort, WorkItemFieldOriginalEstimate,
		WorkItemFieldRemainingWork, WorkItemFieldCompletedWork,
	}

	for _, field := range expectedVSTSSchedulingFields {
		require.Contains(t, operationPaths, field.FieldPath(), "Field %s should be present", field)
	}

	// Verify specific field values
	require.Equal(t, "New", operationPaths[WorkItemFieldState.FieldPath()])
	require.Equal(t, "2", operationPaths[WorkItemFieldPriority.FieldPath()])
	require.Equal(t, "5", operationPaths[WorkItemFieldEffort.FieldPath()])
}

func TestReceiver_GenerateWorkItemDocument_ParseAzureWorkItemFieldIntegration(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	tmpl := template.SimpleTemplate()

	// Test with a mix of fields that have constants and fields that don't
	config := &config.ReceiverConfig{
		Project:     "TestProject",
		IssueType:   "Epic",
		Summary:     `Integration Test`,
		Description: `Testing ParseAzureWorkItemField integration`,
		Fields: map[string]interface{}{
			// Fields with constants (but not Title/Description which are set by Summary/Description)
			"Microsoft.VSTS.Common.Priority": "1",

			// Custom fields without constants
			"Custom.ApplicationName":       "TestApp",
			"Custom.DeploymentEnvironment": "{{ .CommonLabels.env }}",
			"Some.Random.Field":            "random value",
			"":                             "empty field name", // Edge case
		},
	}

	receiver := &Receiver{
		logger: logger,
		conf:   config,
		tmpl:   tmpl,
	}

	data := &alertmanager.Data{
		Status: alertmanager.AlertFiring,
		CommonLabels: alertmanager.KV{
			"env": "staging",
		},
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertFiring,
				Fingerprint: "parse-integration-test",
			},
		},
	}

	document, err := receiver.generateWorkItemDocument(data, true)

	require.NoError(t, err)
	require.NotEmpty(t, document)

	// Verify that fields with constants use the FieldPath() method correctly
	hasSystemTitle := false
	hasSystemDescription := false
	hasPriority := false
	hasCustomApp := false
	hasCustomEnv := false
	hasRandomField := false
	hasEmptyField := false
	hasTags := false

	for _, op := range document {
		if op.Path == nil {
			continue
		}

		switch *op.Path {
		case WorkItemFieldTitle.FieldPath():
			hasSystemTitle = true
			require.Equal(t, "Integration Test", op.Value)
		case WorkItemFieldDescription.FieldPath():
			hasSystemDescription = true
			require.Equal(t, "Testing ParseAzureWorkItemField integration", op.Value)
		case WorkItemFieldPriority.FieldPath():
			hasPriority = true
			require.Equal(t, "1", op.Value)
		case "/fields/Custom.ApplicationName":
			hasCustomApp = true
			require.Equal(t, "TestApp", op.Value)
		case "/fields/Custom.DeploymentEnvironment":
			hasCustomEnv = true
			require.Equal(t, "staging", op.Value)
		case "/fields/Some.Random.Field":
			hasRandomField = true
			require.Equal(t, "random value", op.Value)
		case "/fields/":
			hasEmptyField = true
			require.Equal(t, "empty field name", op.Value)
		case WorkItemFieldTags.FieldPath():
			hasTags = true
			tags := op.Value.(string)
			require.Contains(t, tags, "Fingerprint:parse-integration-test")
		}
	}

	// Verify all expected operations were found
	require.True(t, hasSystemTitle, "System.Title operation should be present")
	require.True(t, hasSystemDescription, "System.Description operation should be present")
	require.True(t, hasPriority, "Microsoft.VSTS.Common.Priority operation should be present")
	require.True(t, hasCustomApp, "Custom.ApplicationName operation should be present")
	require.True(t, hasCustomEnv, "Custom.DeploymentEnvironment operation should be present")
	require.True(t, hasRandomField, "Some.Random.Field operation should be present")
	require.True(t, hasEmptyField, "Empty field operation should be present")
	require.True(t, hasTags, "System.Tags operation should be present")
}

func TestReceiver_GenerateWorkItemDocument_NoFingerprint(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	tmpl := template.SimpleTemplate()
	config := testReceiverConfig1()

	receiver := &Receiver{
		logger: logger,
		conf:   config,
		tmpl:   tmpl,
	}

	data := &alertmanager.Data{
		Alerts: alertmanager.Alerts{
			alertmanager.Alert{
				Status:      alertmanager.AlertResolved,
				Fingerprint: "test-fingerprint",
			},
		},
		Status: alertmanager.AlertResolved,
		GroupLabels: alertmanager.KV{
			"alertname": "TestAlert",
		},
	}

	document, err := receiver.generateWorkItemDocument(data, false)

	require.NoError(t, err)
	require.NotEmpty(t, document)

	// Check that fingerprint tag is not added when addFingerprint is false
	hasFingerprint := false
	for _, op := range document {
		if op.Path != nil && *op.Path == "/fields/System.Tags" {
			hasFingerprint = true
		}
	}
	require.False(t, hasFingerprint)
}
