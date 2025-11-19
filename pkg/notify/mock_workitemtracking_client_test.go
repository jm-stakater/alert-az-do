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
	"io"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/pkg/errors"
)

// mockWorkItemTrackingClient implements the full workitemtracking.Client interface for testing
type mockWorkItemTrackingClient struct {
	workItems       map[int]*workitemtracking.WorkItem
	nextID          int
	workItemsByTag  map[string][]*workitemtracking.WorkItem
	createCalls     []mockCreateCall
	updateCalls     []mockUpdateCall
	queryCalls      []string
	
	// Error control flags for testing error paths
	shouldFailCreate  bool
	shouldFailUpdate  bool
	shouldFailQuery   bool
	duplicateResults  bool
}

type mockCreateCall struct {
	args workitemtracking.CreateWorkItemArgs
}

type mockUpdateCall struct {
	args workitemtracking.UpdateWorkItemArgs
}

func newMockWorkItemTrackingClient() *mockWorkItemTrackingClient {
	return &mockWorkItemTrackingClient{
		workItems:      make(map[int]*workitemtracking.WorkItem),
		workItemsByTag: make(map[string][]*workitemtracking.WorkItem),
		nextID:         1,
		shouldFailCreate: false,
		shouldFailUpdate: false,
		shouldFailQuery:  false,
		duplicateResults: false,
	}
}

// Implement all required methods from workitemtracking.Client interface

func (m *mockWorkItemTrackingClient) CreateWorkItem(ctx context.Context, args workitemtracking.CreateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	m.createCalls = append(m.createCalls, mockCreateCall{args: args})

	// Check if we should fail
	if m.shouldFailCreate {
		return nil, errors.New("mock create work item failed")
	}

	workItem := &workitemtracking.WorkItem{
		Id:     &m.nextID,
		Fields: &map[string]interface{}{},
	}

	// Process the document to set fields
	for _, op := range *args.Document {
		if op.Path != nil {
			switch *op.Path {
			case "/fields/System.Title":
				(*workItem.Fields)["System.Title"] = op.Value
			case "/fields/System.Description":
				(*workItem.Fields)["System.Description"] = op.Value
			case "/fields/System.Tags":
				(*workItem.Fields)["System.Tags"] = op.Value
				// Index by tags for querying
				if tagValue, ok := op.Value.(string); ok {
					m.workItemsByTag[tagValue] = append(m.workItemsByTag[tagValue], workItem)
				}
			default:
				// Handle custom fields
				if len(*op.Path) > 8 && (*op.Path)[:8] == "/fields/" {
					(*workItem.Fields)[(*op.Path)[8:]] = op.Value
				}
			}
		}
	}

	(*workItem.Fields)["System.WorkItemType"] = *args.Type
	(*workItem.Fields)["System.TeamProject"] = *args.Project
	(*workItem.Fields)["System.State"] = "New"

	m.workItems[m.nextID] = workItem
	m.nextID++

	return workItem, nil
}

func (m *mockWorkItemTrackingClient) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	m.updateCalls = append(m.updateCalls, mockUpdateCall{args: args})

	// Check if we should fail
	if m.shouldFailUpdate {
		return nil, errors.New("mock update work item failed")
	}

	workItem, exists := m.workItems[*args.Id]
	if !exists {
		return nil, errors.Errorf("work item %d not found", *args.Id)
	}

	// Process the document to update fields
	for _, op := range *args.Document {
		if op.Path != nil {
			switch *op.Path {
			case "/fields/System.Title":
				(*workItem.Fields)["System.Title"] = op.Value
			case "/fields/System.Description":
				(*workItem.Fields)["System.Description"] = op.Value
			case "/fields/System.State":
				(*workItem.Fields)["System.State"] = op.Value
			default:
				// Handle custom fields
				if len(*op.Path) > 8 && (*op.Path)[:8] == "/fields/" {
					(*workItem.Fields)[(*op.Path)[8:]] = op.Value
				}
			}
		}
	}

	return workItem, nil
}

func (m *mockWorkItemTrackingClient) QueryByWiql(ctx context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
	m.queryCalls = append(m.queryCalls, *args.Wiql.Query)

	// Check if we should fail
	if m.shouldFailQuery {
		return nil, errors.New("mock query failed")
	}

	// Simple mock: extract fingerprint from WIQL query
	var workItems []workitemtracking.WorkItemReference

	// Look for fingerprint in the query
	for tag, items := range m.workItemsByTag {
		if containsSubstring(*args.Wiql.Query, tag) {
			for _, item := range items {
				workItems = append(workItems, workitemtracking.WorkItemReference{
					Id: item.Id,
				})
			}
		}
	}

	// Handle duplicate results scenario for testing
	if m.duplicateResults && len(workItems) > 0 {
		// Add a duplicate
		workItems = append(workItems, workItems[0])
	}

	return &workitemtracking.WorkItemQueryResult{
		WorkItems: &workItems,
	}, nil
}

func (m *mockWorkItemTrackingClient) GetWorkItem(ctx context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
	workItem, exists := m.workItems[*args.Id]
	if !exists {
		return nil, errors.Errorf("work item %d not found", *args.Id)
	}
	return workItem, nil
}

// Mock implementations for other required interface methods (not used in tests but required for interface compliance)
// region Other Methods
// [Preview API] Add a comment on a work item.
func (m *mockWorkItemTrackingClient) AddComment(ctx context.Context, args workitemtracking.AddCommentArgs) (*workitemtracking.Comment, error) {
	return &workitemtracking.Comment{}, nil
}

// [Preview API] Add a comment on a work item.
func (m *mockWorkItemTrackingClient) AddWorkItemComment(ctx context.Context, args workitemtracking.AddWorkItemCommentArgs) (*workitemtracking.Comment, error) {
	return &workitemtracking.Comment{}, nil
}

// [Preview API] Uploads an attachment.
func (m *mockWorkItemTrackingClient) CreateAttachment(ctx context.Context, args workitemtracking.CreateAttachmentArgs) (*workitemtracking.AttachmentReference, error) {
	return &workitemtracking.AttachmentReference{}, nil
}

// [Preview API] Adds a new reaction to a comment.
func (m *mockWorkItemTrackingClient) CreateCommentReaction(ctx context.Context, args workitemtracking.CreateCommentReactionArgs) (*workitemtracking.CommentReaction, error) {
	return &workitemtracking.CommentReaction{}, nil
}

// [Preview API] Create new or update an existing classification node.
func (m *mockWorkItemTrackingClient) CreateOrUpdateClassificationNode(ctx context.Context, args workitemtracking.CreateOrUpdateClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
	return &workitemtracking.WorkItemClassificationNode{}, nil
}

// [Preview API] Creates a query, or moves a query.
func (m *mockWorkItemTrackingClient) CreateQuery(ctx context.Context, args workitemtracking.CreateQueryArgs) (*workitemtracking.QueryHierarchyItem, error) {
	return &workitemtracking.QueryHierarchyItem{}, nil
}

// [Preview API] Creates a template
func (m *mockWorkItemTrackingClient) CreateTemplate(ctx context.Context, args workitemtracking.CreateTemplateArgs) (*workitemtracking.WorkItemTemplate, error) {
	return &workitemtracking.WorkItemTemplate{}, nil
}

// [Preview API] Creates a temporary query
func (m *mockWorkItemTrackingClient) CreateTempQuery(ctx context.Context, args workitemtracking.CreateTempQueryArgs) (*workitemtracking.TemporaryQueryResponseModel, error) {
	return &workitemtracking.TemporaryQueryResponseModel{}, nil
}

// [Preview API] Creates a single work item.
//
//	func (m *mockWorkItemTrackingClient) CreateWorkItem(ctx context.Context, args workitemtracking.CreateWorkItemArgs) (*workitemtracking.WorkItem, error) {
//		return &workitemtracking.WorkItem{}, nil
//	}
//
// [Preview API] Create a new field.
func (m *mockWorkItemTrackingClient) CreateWorkItemField(ctx context.Context, args workitemtracking.CreateWorkItemFieldArgs) (*workitemtracking.WorkItemField2, error) {
	return &workitemtracking.WorkItemField2{}, nil
}

// [Preview API] Delete an existing classification node.
func (m *mockWorkItemTrackingClient) DeleteClassificationNode(ctx context.Context, args workitemtracking.DeleteClassificationNodeArgs) error {
	return nil
}

// [Preview API] Delete a comment on a work item.
func (m *mockWorkItemTrackingClient) DeleteComment(ctx context.Context, args workitemtracking.DeleteCommentArgs) error {
	return nil
}

// [Preview API] Deletes an existing reaction on a comment.
func (m *mockWorkItemTrackingClient) DeleteCommentReaction(ctx context.Context, args workitemtracking.DeleteCommentReactionArgs) (*workitemtracking.CommentReaction, error) {
	return &workitemtracking.CommentReaction{}, nil
}

// [Preview API] Delete a query or a folder. This deletes any permission change on the deleted query or folder and any of its descendants if it is a folder. It is important to note that the deleted permission changes cannot be recovered upon undeleting the query or folder.
func (m *mockWorkItemTrackingClient) DeleteQuery(ctx context.Context, args workitemtracking.DeleteQueryArgs) error {
	return nil
}

// [Preview API]
func (m *mockWorkItemTrackingClient) DeleteTag(ctx context.Context, args workitemtracking.DeleteTagArgs) error {
	return nil
}

// [Preview API] Deletes the template with given id
func (m *mockWorkItemTrackingClient) DeleteTemplate(ctx context.Context, args workitemtracking.DeleteTemplateArgs) error {
	return nil
}

// [Preview API] Deletes the specified work item and sends it to the Recycle Bin, so that it can be restored back, if required. Optionally, if the destroy parameter has been set to true, it destroys the work item permanently. WARNING: If the destroy parameter is set to true, work items deleted by this command will NOT go to recycle-bin and there is no way to restore/recover them after deletion. It is recommended NOT to use this parameter. If you do, please use this parameter with extreme caution.
func (m *mockWorkItemTrackingClient) DeleteWorkItem(ctx context.Context, args workitemtracking.DeleteWorkItemArgs) (*workitemtracking.WorkItemDelete, error) {
	return &workitemtracking.WorkItemDelete{}, nil
}

// [Preview API] Deletes the field. To undelete a filed, see "Update Field" API.
func (m *mockWorkItemTrackingClient) DeleteWorkItemField(ctx context.Context, args workitemtracking.DeleteWorkItemFieldArgs) error {
	return nil
}

// [Preview API] Deletes specified work items and sends them to the Recycle Bin, so that it can be restored back, if required. Optionally, if the destroy parameter has been set to true, it destroys the work item permanently. WARNING: If the destroy parameter is set to true, work items deleted by this command will NOT go to recycle-bin and there is no way to restore/recover them after deletion.
func (m *mockWorkItemTrackingClient) DeleteWorkItems(ctx context.Context, args workitemtracking.DeleteWorkItemsArgs) (*workitemtracking.WorkItemDeleteBatch, error) {
	return &workitemtracking.WorkItemDeleteBatch{}, nil
}

// [Preview API] Destroys the specified work item permanently from the Recycle Bin. This action can not be undone.
func (m *mockWorkItemTrackingClient) DestroyWorkItem(ctx context.Context, args workitemtracking.DestroyWorkItemArgs) error {
	return nil
}

// [Preview API] Downloads an attachment.
func (m *mockWorkItemTrackingClient) GetAttachmentContent(ctx context.Context, args workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error) {
	return nil, nil
}

// [Preview API] Downloads an attachment.
func (m *mockWorkItemTrackingClient) GetAttachmentZip(ctx context.Context, args workitemtracking.GetAttachmentZipArgs) (io.ReadCloser, error) {
	return nil, nil
}

// [Preview API] Gets the classification node for a given node path.
func (m *mockWorkItemTrackingClient) GetClassificationNode(ctx context.Context, args workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
	return &workitemtracking.WorkItemClassificationNode{}, nil
}

// [Preview API] Gets root classification nodes or list of classification nodes for a given list of nodes ids, for a given project. In case ids parameter is supplied you will  get list of classification nodes for those ids. Otherwise you will get root classification nodes for this project.
func (m *mockWorkItemTrackingClient) GetClassificationNodes(ctx context.Context, args workitemtracking.GetClassificationNodesArgs) (*[]workitemtracking.WorkItemClassificationNode, error) {
	return &[]workitemtracking.WorkItemClassificationNode{}, nil
}

// [Preview API] Returns a work item comment.
func (m *mockWorkItemTrackingClient) GetComment(ctx context.Context, args workitemtracking.GetCommentArgs) (*workitemtracking.Comment, error) {
	return &workitemtracking.Comment{}, nil
}

// [Preview API] Gets reactions of a comment.
func (m *mockWorkItemTrackingClient) GetCommentReactions(ctx context.Context, args workitemtracking.GetCommentReactionsArgs) (*[]workitemtracking.CommentReaction, error) {
	return &[]workitemtracking.CommentReaction{}, nil
}

// [Preview API] Returns a list of work item comments, pageable.
func (m *mockWorkItemTrackingClient) GetComments(ctx context.Context, args workitemtracking.GetCommentsArgs) (*workitemtracking.CommentList, error) {
	return &workitemtracking.CommentList{}, nil
}

// [Preview API] Returns a list of work item comments by ids.
func (m *mockWorkItemTrackingClient) GetCommentsBatch(ctx context.Context, args workitemtracking.GetCommentsBatchArgs) (*workitemtracking.CommentList, error) {
	return &workitemtracking.CommentList{}, nil
}

// [Preview API]
func (m *mockWorkItemTrackingClient) GetCommentVersion(ctx context.Context, args workitemtracking.GetCommentVersionArgs) (*workitemtracking.CommentVersion, error) {
	return &workitemtracking.CommentVersion{}, nil
}

// [Preview API]
func (m *mockWorkItemTrackingClient) GetCommentVersions(ctx context.Context, args workitemtracking.GetCommentVersionsArgs) (*[]workitemtracking.CommentVersion, error) {
	return &[]workitemtracking.CommentVersion{}, nil
}

// [Preview API] Gets a deleted work item from Recycle Bin.
func (m *mockWorkItemTrackingClient) GetDeletedWorkItem(ctx context.Context, args workitemtracking.GetDeletedWorkItemArgs) (*workitemtracking.WorkItemDelete, error) {
	return &workitemtracking.WorkItemDelete{}, nil
}

// [Preview API] Gets the work items from the recycle bin, whose IDs have been specified in the parameters
func (m *mockWorkItemTrackingClient) GetDeletedWorkItems(ctx context.Context, args workitemtracking.GetDeletedWorkItemsArgs) (*[]workitemtracking.WorkItemDeleteReference, error) {
	return &[]workitemtracking.WorkItemDeleteReference{}, nil
}

// [Preview API] Gets a list of the IDs and the URLs of the deleted the work items in the Recycle Bin.
func (m *mockWorkItemTrackingClient) GetDeletedWorkItemShallowReferences(ctx context.Context, args workitemtracking.GetDeletedWorkItemShallowReferencesArgs) (*[]workitemtracking.WorkItemDeleteShallowReference, error) {
	return &[]workitemtracking.WorkItemDeleteShallowReference{}, nil
}

// [Preview API] Get users who reacted on the comment.
func (m *mockWorkItemTrackingClient) GetEngagedUsers(ctx context.Context, args workitemtracking.GetEngagedUsersArgs) (*[]webapi.IdentityRef, error) {
	return &[]webapi.IdentityRef{}, nil
}

// [Preview API] Gets a list of repos within specified github connection.
func (m *mockWorkItemTrackingClient) GetGithubConnectionRepositories(ctx context.Context, args workitemtracking.GetGithubConnectionRepositoriesArgs) (*[]workitemtracking.GitHubConnectionRepoModel, error) {
	return &[]workitemtracking.GitHubConnectionRepoModel{}, nil
}

// [Preview API] Gets a list of github connections
func (m *mockWorkItemTrackingClient) GetGithubConnections(ctx context.Context, args workitemtracking.GetGithubConnectionsArgs) (*[]workitemtracking.GitHubConnectionModel, error) {
	return &[]workitemtracking.GitHubConnectionModel{}, nil
}

// [Preview API] Gets the root queries and their children
func (m *mockWorkItemTrackingClient) GetQueries(ctx context.Context, args workitemtracking.GetQueriesArgs) (*[]workitemtracking.QueryHierarchyItem, error) {
	return &[]workitemtracking.QueryHierarchyItem{}, nil
}

// [Preview API] Gets a list of queries by ids (Maximum 1000)
func (m *mockWorkItemTrackingClient) GetQueriesBatch(ctx context.Context, args workitemtracking.GetQueriesBatchArgs) (*[]workitemtracking.QueryHierarchyItem, error) {
	return &[]workitemtracking.QueryHierarchyItem{}, nil
}

// [Preview API] Retrieves an individual query and its children
func (m *mockWorkItemTrackingClient) GetQuery(ctx context.Context, args workitemtracking.GetQueryArgs) (*workitemtracking.QueryHierarchyItem, error) {
	return &workitemtracking.QueryHierarchyItem{}, nil
}

// [Preview API] Gets the results of the query given the query ID.
func (m *mockWorkItemTrackingClient) GetQueryResultCount(ctx context.Context, args workitemtracking.GetQueryResultCountArgs) (*int, error) {
	return nil, nil
}

// [Preview API] Gets recent work item activities
func (m *mockWorkItemTrackingClient) GetRecentActivityData(ctx context.Context, args workitemtracking.GetRecentActivityDataArgs) (*[]workitemtracking.AccountRecentActivityWorkItemModel2, error) {
	return &[]workitemtracking.AccountRecentActivityWorkItemModel2{}, nil
}

// [Preview API] Gets the work item relation type definition.
func (m *mockWorkItemTrackingClient) GetRelationType(ctx context.Context, args workitemtracking.GetRelationTypeArgs) (*workitemtracking.WorkItemRelationType, error) {
	return &workitemtracking.WorkItemRelationType{}, nil
}

// [Preview API] Gets the work item relation types.
func (m *mockWorkItemTrackingClient) GetRelationTypes(ctx context.Context, args workitemtracking.GetRelationTypesArgs) (*[]workitemtracking.WorkItemRelationType, error) {
	return &[]workitemtracking.WorkItemRelationType{}, nil
}

// [Preview API] Get a batch of work item links
func (m *mockWorkItemTrackingClient) GetReportingLinksByLinkType(ctx context.Context, args workitemtracking.GetReportingLinksByLinkTypeArgs) (*workitemtracking.ReportingWorkItemLinksBatch, error) {
	return &workitemtracking.ReportingWorkItemLinksBatch{}, nil
}

// [Preview API] Returns a fully hydrated work item for the requested revision
func (m *mockWorkItemTrackingClient) GetRevision(ctx context.Context, args workitemtracking.GetRevisionArgs) (*workitemtracking.WorkItem, error) {
	return &workitemtracking.WorkItem{}, nil
}

// [Preview API] Returns the list of fully hydrated work item revisions, paged.
func (m *mockWorkItemTrackingClient) GetRevisions(ctx context.Context, args workitemtracking.GetRevisionsArgs) (*[]workitemtracking.WorkItem, error) {
	return &[]workitemtracking.WorkItem{}, nil
}

// [Preview API] Gets root classification nodes under the project.
func (m *mockWorkItemTrackingClient) GetRootNodes(ctx context.Context, args workitemtracking.GetRootNodesArgs) (*[]workitemtracking.WorkItemClassificationNode, error) {
	return &[]workitemtracking.WorkItemClassificationNode{}, nil
}

// [Preview API]
func (m *mockWorkItemTrackingClient) GetTag(ctx context.Context, args workitemtracking.GetTagArgs) (*workitemtracking.WorkItemTagDefinition, error) {
	return &workitemtracking.WorkItemTagDefinition{}, nil
}

// [Preview API]
func (m *mockWorkItemTrackingClient) GetTags(ctx context.Context, args workitemtracking.GetTagsArgs) (*[]workitemtracking.WorkItemTagDefinition, error) {
	return &[]workitemtracking.WorkItemTagDefinition{}, nil
}

// [Preview API] Gets the template with specified id
func (m *mockWorkItemTrackingClient) GetTemplate(ctx context.Context, args workitemtracking.GetTemplateArgs) (*workitemtracking.WorkItemTemplate, error) {
	return &workitemtracking.WorkItemTemplate{}, nil
}

// [Preview API] Gets template
func (m *mockWorkItemTrackingClient) GetTemplates(ctx context.Context, args workitemtracking.GetTemplatesArgs) (*[]workitemtracking.WorkItemTemplateReference, error) {
	return &[]workitemtracking.WorkItemTemplateReference{}, nil
}

// [Preview API] Returns a single update for a work item
func (m *mockWorkItemTrackingClient) GetUpdate(ctx context.Context, args workitemtracking.GetUpdateArgs) (*workitemtracking.WorkItemUpdate, error) {
	return &workitemtracking.WorkItemUpdate{}, nil
}

// [Preview API] Returns a the deltas between work item revisions
func (m *mockWorkItemTrackingClient) GetUpdates(ctx context.Context, args workitemtracking.GetUpdatesArgs) (*[]workitemtracking.WorkItemUpdate, error) {
	return &[]workitemtracking.WorkItemUpdate{}, nil
}

// [Preview API] Get the list of work item tracking outbound artifact link types.
func (m *mockWorkItemTrackingClient) GetWorkArtifactLinkTypes(ctx context.Context, args workitemtracking.GetWorkArtifactLinkTypesArgs) (*[]workitemtracking.WorkArtifactLink, error) {
	return &[]workitemtracking.WorkArtifactLink{}, nil
}

// [Preview API] Returns a single work item.
//
//	func (m *mockWorkItemTrackingClient) GetWorkItem(ctx context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
//		return &workitemtracking.WorkItem{}, nil
//	}
//
// [Preview API] Gets information on a specific field.
func (m *mockWorkItemTrackingClient) GetWorkItemField(ctx context.Context, args workitemtracking.GetWorkItemFieldArgs) (*workitemtracking.WorkItemField2, error) {
	return &workitemtracking.WorkItemField2{}, nil
}

// [Preview API] Returns information for all fields. The project ID/name parameter is optional.
func (m *mockWorkItemTrackingClient) GetWorkItemFields(ctx context.Context, args workitemtracking.GetWorkItemFieldsArgs) (*[]workitemtracking.WorkItemField2, error) {
	return &[]workitemtracking.WorkItemField2{}, nil
}

// [Preview API] Get a work item icon given the friendly name and icon color.
func (m *mockWorkItemTrackingClient) GetWorkItemIconJson(ctx context.Context, args workitemtracking.GetWorkItemIconJsonArgs) (*workitemtracking.WorkItemIcon, error) {
	return &workitemtracking.WorkItemIcon{}, nil
}

// [Preview API] Get a list of all work item icons.
func (m *mockWorkItemTrackingClient) GetWorkItemIcons(ctx context.Context, args workitemtracking.GetWorkItemIconsArgs) (*[]workitemtracking.WorkItemIcon, error) {
	return &[]workitemtracking.WorkItemIcon{}, nil
}

// [Preview API] Get a work item icon given the friendly name and icon color.
func (m *mockWorkItemTrackingClient) GetWorkItemIconSvg(ctx context.Context, args workitemtracking.GetWorkItemIconSvgArgs) (io.ReadCloser, error) {
	return nil, nil
}

// [Preview API] Get a work item icon given the friendly name and icon color.
func (m *mockWorkItemTrackingClient) GetWorkItemIconXaml(ctx context.Context, args workitemtracking.GetWorkItemIconXamlArgs) (io.ReadCloser, error) {
	return nil, nil
}

// [Preview API] Returns the next state on the given work item IDs.
func (m *mockWorkItemTrackingClient) GetWorkItemNextStatesOnCheckinAction(ctx context.Context, args workitemtracking.GetWorkItemNextStatesOnCheckinActionArgs) (*[]workitemtracking.WorkItemNextStateOnTransition, error) {
	return &[]workitemtracking.WorkItemNextStateOnTransition{}, nil
}

// [Preview API] Returns a list of work items (Maximum 200)
func (m *mockWorkItemTrackingClient) GetWorkItems(ctx context.Context, args workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
	return &[]workitemtracking.WorkItem{}, nil
}

// [Preview API] Gets work items for a list of work item ids (Maximum 200)
func (m *mockWorkItemTrackingClient) GetWorkItemsBatch(ctx context.Context, args workitemtracking.GetWorkItemsBatchArgs) (*[]workitemtracking.WorkItem, error) {
	return &[]workitemtracking.WorkItem{}, nil
}

// [Preview API] Returns a single work item from a template.
func (m *mockWorkItemTrackingClient) GetWorkItemTemplate(ctx context.Context, args workitemtracking.GetWorkItemTemplateArgs) (*workitemtracking.WorkItem, error) {
	return &workitemtracking.WorkItem{}, nil
}

// [Preview API] Returns a work item type definition.
func (m *mockWorkItemTrackingClient) GetWorkItemType(ctx context.Context, args workitemtracking.GetWorkItemTypeArgs) (*workitemtracking.WorkItemType, error) {
	return &workitemtracking.WorkItemType{}, nil
}

// [Preview API] Get all work item type categories.
func (m *mockWorkItemTrackingClient) GetWorkItemTypeCategories(ctx context.Context, args workitemtracking.GetWorkItemTypeCategoriesArgs) (*[]workitemtracking.WorkItemTypeCategory, error) {
	return &[]workitemtracking.WorkItemTypeCategory{}, nil
}

// [Preview API] Get specific work item type category by name.
func (m *mockWorkItemTrackingClient) GetWorkItemTypeCategory(ctx context.Context, args workitemtracking.GetWorkItemTypeCategoryArgs) (*workitemtracking.WorkItemTypeCategory, error) {
	return &workitemtracking.WorkItemTypeCategory{}, nil
}

// [Preview API] Get a list of fields for a work item type with detailed references.
func (m *mockWorkItemTrackingClient) GetWorkItemTypeFieldsWithReferences(ctx context.Context, args workitemtracking.GetWorkItemTypeFieldsWithReferencesArgs) (*[]workitemtracking.WorkItemTypeFieldWithReferences, error) {
	return &[]workitemtracking.WorkItemTypeFieldWithReferences{}, nil
}

// [Preview API] Get a field for a work item type with detailed references.
func (m *mockWorkItemTrackingClient) GetWorkItemTypeFieldWithReferences(ctx context.Context, args workitemtracking.GetWorkItemTypeFieldWithReferencesArgs) (*workitemtracking.WorkItemTypeFieldWithReferences, error) {
	return &workitemtracking.WorkItemTypeFieldWithReferences{}, nil
}

// [Preview API] Returns the list of work item types
func (m *mockWorkItemTrackingClient) GetWorkItemTypes(ctx context.Context, args workitemtracking.GetWorkItemTypesArgs) (*[]workitemtracking.WorkItemType, error) {
	return &[]workitemtracking.WorkItemType{}, nil
}

// [Preview API] Returns the state names and colors for a work item type.
func (m *mockWorkItemTrackingClient) GetWorkItemTypeStates(ctx context.Context, args workitemtracking.GetWorkItemTypeStatesArgs) (*[]workitemtracking.WorkItemStateColor, error) {
	return &[]workitemtracking.WorkItemStateColor{}, nil
}

// [Preview API] Migrates a project to a different process within the same OOB type. For example, you can only migrate a project from agile/custom-agile to agile/custom-agile.
func (m *mockWorkItemTrackingClient) MigrateProjectsProcess(ctx context.Context, args workitemtracking.MigrateProjectsProcessArgs) (*workitemtracking.ProcessMigrationResultModel, error) {
	return &workitemtracking.ProcessMigrationResultModel{}, nil
}

// [Preview API] Gets the results of the query given the query ID.
func (m *mockWorkItemTrackingClient) QueryById(ctx context.Context, args workitemtracking.QueryByIdArgs) (*workitemtracking.WorkItemQueryResult, error) {
	return &workitemtracking.WorkItemQueryResult{}, nil
}

// [Preview API] Gets the results of the query given its WIQL.
//
//	func (m *mockWorkItemTrackingClient) QueryByWiql(ctx context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
//		return &workitemtracking.WorkItemQueryResult{}, nil
//	}
//
// [Preview API] Queries work items linked to a given list of artifact URI.
func (m *mockWorkItemTrackingClient) QueryWorkItemsForArtifactUris(ctx context.Context, args workitemtracking.QueryWorkItemsForArtifactUrisArgs) (*workitemtracking.ArtifactUriQueryResult, error) {
	return &workitemtracking.ArtifactUriQueryResult{}, nil
}

// [Preview API]
func (m *mockWorkItemTrackingClient) ReadReportingDiscussions(ctx context.Context, args workitemtracking.ReadReportingDiscussionsArgs) (*workitemtracking.ReportingWorkItemRevisionsBatch, error) {
	return &workitemtracking.ReportingWorkItemRevisionsBatch{}, nil
}

// [Preview API] Get a batch of work item revisions with the option of including deleted items
func (m *mockWorkItemTrackingClient) ReadReportingRevisionsGet(ctx context.Context, args workitemtracking.ReadReportingRevisionsGetArgs) (*workitemtracking.ReportingWorkItemRevisionsBatch, error) {
	return &workitemtracking.ReportingWorkItemRevisionsBatch{}, nil
}

// [Preview API] Get a batch of work item revisions. This request may be used if your list of fields is large enough that it may run the URL over the length limit.
func (m *mockWorkItemTrackingClient) ReadReportingRevisionsPost(ctx context.Context, args workitemtracking.ReadReportingRevisionsPostArgs) (*workitemtracking.ReportingWorkItemRevisionsBatch, error) {
	return &workitemtracking.ReportingWorkItemRevisionsBatch{}, nil
}

// [Preview API] Replace template contents
func (m *mockWorkItemTrackingClient) ReplaceTemplate(ctx context.Context, args workitemtracking.ReplaceTemplateArgs) (*workitemtracking.WorkItemTemplate, error) {
	return &workitemtracking.WorkItemTemplate{}, nil
}

// [Preview API] Restores the deleted work item from Recycle Bin.
func (m *mockWorkItemTrackingClient) RestoreWorkItem(ctx context.Context, args workitemtracking.RestoreWorkItemArgs) (*workitemtracking.WorkItemDelete, error) {
	return &workitemtracking.WorkItemDelete{}, nil
}

// [Preview API] Searches all queries the user has access to in the current project
func (m *mockWorkItemTrackingClient) SearchQueries(ctx context.Context, args workitemtracking.SearchQueriesArgs) (*workitemtracking.QueryHierarchyItemsResult, error) {
	return &workitemtracking.QueryHierarchyItemsResult{}, nil
}

// [Preview API] RESTful method to send mail for selected/queried work items.
func (m *mockWorkItemTrackingClient) SendMail(ctx context.Context, args workitemtracking.SendMailArgs) error {
	return nil
}

// [Preview API] Update an existing classification node.
func (m *mockWorkItemTrackingClient) UpdateClassificationNode(ctx context.Context, args workitemtracking.UpdateClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
	return &workitemtracking.WorkItemClassificationNode{}, nil
}

// [Preview API] Update a comment on a work item.
func (m *mockWorkItemTrackingClient) UpdateComment(ctx context.Context, args workitemtracking.UpdateCommentArgs) (*workitemtracking.Comment, error) {
	return &workitemtracking.Comment{}, nil
}

// [Preview API] Add/remove list of repos within specified github connection.
func (m *mockWorkItemTrackingClient) UpdateGithubConnectionRepos(ctx context.Context, args workitemtracking.UpdateGithubConnectionReposArgs) (*[]workitemtracking.GitHubConnectionRepoModel, error) {
	return &[]workitemtracking.GitHubConnectionRepoModel{}, nil
}

// [Preview API] Update a query or a folder. This allows you to update, rename and move queries and folders.
func (m *mockWorkItemTrackingClient) UpdateQuery(ctx context.Context, args workitemtracking.UpdateQueryArgs) (*workitemtracking.QueryHierarchyItem, error) {
	return &workitemtracking.QueryHierarchyItem{}, nil
}

// [Preview API]
func (m *mockWorkItemTrackingClient) UpdateTag(ctx context.Context, args workitemtracking.UpdateTagArgs) (*workitemtracking.WorkItemTagDefinition, error) {
	return &workitemtracking.WorkItemTagDefinition{}, nil
}

// [Preview API] Updates a single work item.
//
//	func (m *mockWorkItemTrackingClient) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
//		return &workitemtracking.WorkItem{}, nil
//	}
//
// [Preview API] Update a comment on a work item.
func (m *mockWorkItemTrackingClient) UpdateWorkItemComment(ctx context.Context, args workitemtracking.UpdateWorkItemCommentArgs) (*workitemtracking.Comment, error) {
	return &workitemtracking.Comment{}, nil
}

// [Preview API] Update a field.
func (m *mockWorkItemTrackingClient) UpdateWorkItemField(ctx context.Context, args workitemtracking.UpdateWorkItemFieldArgs) (*workitemtracking.WorkItemField2, error) {
	return &workitemtracking.WorkItemField2{}, nil
}

//endregion
