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
