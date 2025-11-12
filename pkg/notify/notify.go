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

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/jm-stakater/alert-az-do/pkg/alertmanager"
	"github.com/jm-stakater/alert-az-do/pkg/config"
	"github.com/jm-stakater/alert-az-do/pkg/template"
	v7 "github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/pkg/errors"
)

// Receiver wraps Azure DevOps client with configuration
type Receiver struct {
	logger log.Logger
	client workitemtracking.Client
	conf   *config.ReceiverConfig
	tmpl   *template.Template
}

// NewReceiver creates a new Azure DevOps receiver
func NewReceiver(ctx context.Context, logger log.Logger, c *config.ReceiverConfig, t *template.Template, connection *v7.Connection) *Receiver {
	client, err := workitemtracking.NewClient(ctx, connection)
	if err != nil {
		level.Error(logger).Log("msg", "failed to create Azure DevOps work item tracking client", "err", err)
		return nil
	}
	return &Receiver{
		logger: logger,
		conf:   c,
		tmpl:   t,
		client: client,
	}
}

// Notify processes alerts and creates/updates Azure DevOps work items
func (r *Receiver) Notify(ctx context.Context, data *alertmanager.Data) error {
	project, err := r.tmpl.Execute(r.conf.Project, data)
	if err != nil {
		return errors.Wrap(err, "generate project from template")
	}

	if len(data.Alerts.Firing()) > 0 {
		workItemRef, err := r.findWorkItem(ctx, data, project)
		if err != nil {
			return errors.Wrap(err, "find work item")
		}
		if workItemRef != nil {
			level.Info(r.logger).Log("msg", "work item already exists for firing alert", "id", workItemRef.Id)
			return r.updateWorkItem(ctx, data, project, workItemRef)
		}

		// Create new work item for firing alerts
		return r.createWorkItem(ctx, data, project)
	} else {
		// Resolve existing work item
		return r.resolveWorkItem(ctx, data, project)
	}
}

func (r *Receiver) updateWorkItem(ctx context.Context, data *alertmanager.Data, project string, workItemRef *workitemtracking.WorkItem) error {
	document, err := r.generateWorkItemDocument(data, true)
	if err != nil {
		return errors.Wrap(err, "generate work item document")
	}

	payload := workitemtracking.UpdateWorkItemArgs{
		Document:     &document,
		Id:           workItemRef.Id,
		Project:      &project,
		ValidateOnly: nil,
	}

	workItem, err := r.client.UpdateWorkItem(ctx, payload)
	if err != nil {
		return errors.Wrap(err, "update work item")
	}

	level.Info(r.logger).Log("msg", "work item updated", "id", workItem.Id, "title", (*workItem.Fields)["System.Title"].(string))
	return nil
}

func (r *Receiver) createWorkItem(ctx context.Context, data *alertmanager.Data, project string) error {
	workItemType, err := r.tmpl.Execute(r.conf.IssueType, data)
	if err != nil {
		return errors.Wrap(err, "render work item type")
	}

	document, err := r.generateWorkItemDocument(data, true)
	if err != nil {
		return errors.Wrap(err, "generate work item document")
	}

	payload := workitemtracking.CreateWorkItemArgs{
		Document:     &document,
		Project:      &project,
		Type:         &workItemType,
		ValidateOnly: nil,
	}

	workItem, err := r.client.CreateWorkItem(ctx, payload)
	if err != nil {
		return errors.Wrap(err, "create work item")
	}

	level.Info(r.logger).Log("msg", "work item created", "id", workItem.Id, "title", (*workItem.Fields)["System.Title"].(string))
	return nil
}

func (r *Receiver) findWorkItem(ctx context.Context, data *alertmanager.Data, project string) (*workitemtracking.WorkItem, error) {
	if len(data.Alerts) == 0 {
		return nil, errors.New("no alerts in data")
	}

	fingerprint := data.Alerts[0].Fingerprint
	wiql := fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = '%s' AND [System.Tags] CONTAINS 'Fingerprint:%s'", project, fingerprint)

	query := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &wiql,
		},
	}

	queryResult, err := r.client.QueryByWiql(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "query work items")
	}

	if len(*queryResult.WorkItems) == 0 {
		level.Debug(r.logger).Log("msg", "no work items found", "fingerprint", fingerprint)
		return nil, nil
	} else if len(*queryResult.WorkItems) > 1 {
		level.Debug(r.logger).Log("msg", "duplicate fingerprint on work items found", "fingerprint", fingerprint)
		return nil, nil
	}

	workItemRef := (*queryResult.WorkItems)[0]
	return r.client.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:     workItemRef.Id,
		Expand: nil,
	})
}

func (r *Receiver) resolveWorkItem(ctx context.Context, data *alertmanager.Data, project string) error {
	workItemRef, err := r.findWorkItem(ctx, data, project)
	if err != nil {
		return errors.Wrap(err, "find work item")
	}
	if workItemRef == nil {
		level.Info(r.logger).Log("msg", "no work item found to resolve")
		return nil
	}
	document, err := r.generateWorkItemDocument(data, false)
	if err != nil {
		return errors.Wrap(err, "generate resolve document")
	}

	payload := workitemtracking.UpdateWorkItemArgs{
		Document:     &document,
		Id:           workItemRef.Id,
		ValidateOnly: nil,
	}

	workItem, err := r.client.UpdateWorkItem(ctx, payload)
	if err != nil {
		return errors.Wrap(err, "update work item")
	}

	level.Info(r.logger).Log("msg", "work item resolved", "id", workItem.Id, "title", (*workItem.Fields)["System.Title"])
	return nil
}

func (r *Receiver) generateWorkItemDocument(data *alertmanager.Data, addFingerprint bool) ([]webapi.JsonPatchOperation, error) {
	var document []webapi.JsonPatchOperation

	// Add title
	title, err := r.tmpl.Execute(r.conf.Summary, data)
	if err != nil {
		return nil, errors.Wrap(err, "render title")
	}

	systemTitlePath := "/fields/System.Title"
	document = append(document, webapi.JsonPatchOperation{
		Op:    &webapi.OperationValues.Add,
		Path:  &systemTitlePath,
		Value: title,
	})

	// Add description
	description, err := r.tmpl.Execute(r.conf.Description, data)
	if err != nil {
		return nil, errors.Wrap(err, "render description")
	}
	systemDescriptionPath := "/fields/System.Description"
	document = append(document, webapi.JsonPatchOperation{
		Op:    &webapi.OperationValues.Add,
		Path:  &systemDescriptionPath,
		Value: description,
	})

	// Add fingerprint tag if creating new work item
	if addFingerprint && len(data.Alerts) > 0 {
		fingerprint := data.Alerts[0].Fingerprint
		systemTagsPath := "/fields/System.Tags"
		document = append(document, webapi.JsonPatchOperation{
			Op:    &webapi.OperationValues.Add,
			Path:  &systemTagsPath,
			Value: fmt.Sprintf("Fingerprint:%s", fingerprint),
		})
	}

	// Add custom fields from configuration
	for key, value := range r.conf.Fields {
		fieldValue, err := r.tmpl.Execute(fmt.Sprintf("%v", value), data)
		if err != nil {
			return nil, errors.Wrapf(err, "render field %s", key)
		}
		path := fmt.Sprintf("/fields/%s", key)
		document = append(document, webapi.JsonPatchOperation{
			Op:    &webapi.OperationValues.Add,
			Path:  &path,
			Value: fieldValue,
		})
	}

	return document, nil
}
