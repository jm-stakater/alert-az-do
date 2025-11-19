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
	"fmt"
	"strings"
)

type AzureWorkItemField string

// String returns the field reference name as a string
func (f AzureWorkItemField) String() string {
	return string(f)
}

// FieldPath returns the field path formatted for Azure DevOps JSON patch operations
func (f AzureWorkItemField) FieldPath() string {
	return fmt.Sprintf("/fields/%s", string(f))
}

// MarshalJSON implements json.Marshaler interface, returning the field reference name
func (f AzureWorkItemField) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, `"%s"`, string(f)), nil
}

const (
	// System Fields
	WorkItemFieldTitle             AzureWorkItemField = "System.Title"
	WorkItemFieldDescription       AzureWorkItemField = "System.Description"
	WorkItemFieldState             AzureWorkItemField = "System.State"
	WorkItemFieldAreaId            AzureWorkItemField = "System.AreaId"
	WorkItemFieldAreaPath          AzureWorkItemField = "System.AreaPath"
	WorkItemFieldAssignedTo        AzureWorkItemField = "System.AssignedTo"
	WorkItemFieldAttachedFileCount AzureWorkItemField = "System.AttachedFileCount"
	WorkItemFieldAuthorizedAs      AzureWorkItemField = "System.AuthorizedAs"
	WorkItemFieldAuthorizedDate    AzureWorkItemField = "System.AuthorizedDate"
	WorkItemFieldBoardColumn       AzureWorkItemField = "System.BoardColumn"
	WorkItemFieldBoardColumnDone   AzureWorkItemField = "System.BoardColumnDone"
	WorkItemFieldBoardLane         AzureWorkItemField = "System.BoardLane"
	WorkItemFieldChangedBy         AzureWorkItemField = "System.ChangedBy"
	WorkItemFieldChangedDate       AzureWorkItemField = "System.ChangedDate"
	WorkItemFieldCommentCount      AzureWorkItemField = "System.CommentCount"
	WorkItemFieldCreatedBy         AzureWorkItemField = "System.CreatedBy"
	WorkItemFieldCreatedDate       AzureWorkItemField = "System.CreatedDate"
	WorkItemFieldExternalLinkCount AzureWorkItemField = "System.ExternalLinkCount"
	WorkItemFieldHistory           AzureWorkItemField = "System.History"
	WorkItemFieldHyperLinkCount    AzureWorkItemField = "System.HyperLinkCount"
	WorkItemFieldId                AzureWorkItemField = "System.Id"
	WorkItemFieldIterationId       AzureWorkItemField = "System.IterationId"
	WorkItemFieldIterationPath     AzureWorkItemField = "System.IterationPath"
	WorkItemFieldNodeName          AzureWorkItemField = "System.NodeName"
	WorkItemFieldParent            AzureWorkItemField = "System.Parent"
	WorkItemFieldReason            AzureWorkItemField = "System.Reason"
	WorkItemFieldRelatedLinkCount  AzureWorkItemField = "System.RelatedLinkCount"
	WorkItemFieldRemoteLinkCount   AzureWorkItemField = "System.RemoteLinkCount"
	WorkItemFieldRev               AzureWorkItemField = "System.Rev"
	WorkItemFieldRevisedDate       AzureWorkItemField = "System.RevisedDate"
	WorkItemFieldTags              AzureWorkItemField = "System.Tags"
	WorkItemFieldTeamProject       AzureWorkItemField = "System.TeamProject"
	WorkItemFieldWatermark         AzureWorkItemField = "System.Watermark"
	WorkItemFieldWorkItemType      AzureWorkItemField = "System.WorkItemType"

	// Microsoft.VSTS.Common Fields
	WorkItemFieldAcceptanceCriteria AzureWorkItemField = "Microsoft.VSTS.Common.AcceptanceCriteria"
	WorkItemFieldActivatedBy        AzureWorkItemField = "Microsoft.VSTS.Common.ActivatedBy"
	WorkItemFieldActivatedDate      AzureWorkItemField = "Microsoft.VSTS.Common.ActivatedDate"
	WorkItemFieldActivity           AzureWorkItemField = "Microsoft.VSTS.Common.Activity"
	WorkItemFieldBusinessValue      AzureWorkItemField = "Microsoft.VSTS.Common.BusinessValue"
	WorkItemFieldClosedBy           AzureWorkItemField = "Microsoft.VSTS.Common.ClosedBy"
	WorkItemFieldClosedDate         AzureWorkItemField = "Microsoft.VSTS.Common.ClosedDate"
	WorkItemFieldIssue              AzureWorkItemField = "Microsoft.VSTS.Common.Issue"
	WorkItemFieldPriority           AzureWorkItemField = "Microsoft.VSTS.Common.Priority"
	WorkItemFieldRating             AzureWorkItemField = "Microsoft.VSTS.Common.Rating"
	WorkItemFieldResolvedBy         AzureWorkItemField = "Microsoft.VSTS.Common.ResolvedBy"
	WorkItemFieldResolvedDate       AzureWorkItemField = "Microsoft.VSTS.Common.ResolvedDate"
	WorkItemFieldResolvedReason     AzureWorkItemField = "Microsoft.VSTS.Common.ResolvedReason"
	WorkItemFieldReviewedBy         AzureWorkItemField = "Microsoft.VSTS.Common.ReviewedBy"
	WorkItemFieldRisk               AzureWorkItemField = "Microsoft.VSTS.Common.Risk"
	WorkItemFieldSeverity           AzureWorkItemField = "Microsoft.VSTS.Common.Severity"
	WorkItemFieldStackRank          AzureWorkItemField = "Microsoft.VSTS.Common.StackRank"
	WorkItemFieldStateChangeDate    AzureWorkItemField = "Microsoft.VSTS.Common.StateChangeDate"
	WorkItemFieldStateCode          AzureWorkItemField = "Microsoft.VSTS.Common.StateCode"
	WorkItemFieldTimeCriticality    AzureWorkItemField = "Microsoft.VSTS.Common.TimeCriticality"
	WorkItemFieldValueArea          AzureWorkItemField = "Microsoft.VSTS.Common.ValueArea"

	// Microsoft.VSTS.Scheduling Fields
	WorkItemFieldCompletedWork    AzureWorkItemField = "Microsoft.VSTS.Scheduling.CompletedWork"
	WorkItemFieldDueDate          AzureWorkItemField = "Microsoft.VSTS.Scheduling.DueDate"
	WorkItemFieldEffort           AzureWorkItemField = "Microsoft.VSTS.Scheduling.Effort"
	WorkItemFieldFinishDate       AzureWorkItemField = "Microsoft.VSTS.Scheduling.FinishDate"
	WorkItemFieldOriginalEstimate AzureWorkItemField = "Microsoft.VSTS.Scheduling.OriginalEstimate"
	WorkItemFieldRemainingWork    AzureWorkItemField = "Microsoft.VSTS.Scheduling.RemainingWork"
	WorkItemFieldStartDate        AzureWorkItemField = "Microsoft.VSTS.Scheduling.StartDate"
	WorkItemFieldStoryPoints      AzureWorkItemField = "Microsoft.VSTS.Scheduling.StoryPoints"
	WorkItemFieldTargetDate       AzureWorkItemField = "Microsoft.VSTS.Scheduling.TargetDate"

	// Microsoft.VSTS.Build Fields
	WorkItemFieldFoundIn          AzureWorkItemField = "Microsoft.VSTS.Build.FoundIn"
	WorkItemFieldIntegrationBuild AzureWorkItemField = "Microsoft.VSTS.Build.IntegrationBuild"

	// Microsoft.VSTS.CodeReview Fields
	WorkItemFieldAcceptedBy       AzureWorkItemField = "Microsoft.VSTS.CodeReview.AcceptedBy"
	WorkItemFieldAcceptedDate     AzureWorkItemField = "Microsoft.VSTS.CodeReview.AcceptedDate"
	WorkItemFieldClosedStatus     AzureWorkItemField = "Microsoft.VSTS.CodeReview.ClosedStatus"
	WorkItemFieldClosedStatusCode AzureWorkItemField = "Microsoft.VSTS.CodeReview.ClosedStatusCode"
	WorkItemFieldClosingComment   AzureWorkItemField = "Microsoft.VSTS.CodeReview.ClosingComment"
	WorkItemFieldContext          AzureWorkItemField = "Microsoft.VSTS.CodeReview.Context"
	WorkItemFieldContextCode      AzureWorkItemField = "Microsoft.VSTS.CodeReview.ContextCode"
	WorkItemFieldContextOwner     AzureWorkItemField = "Microsoft.VSTS.CodeReview.ContextOwner"
	WorkItemFieldContextType      AzureWorkItemField = "Microsoft.VSTS.CodeReview.ContextType"

	// Microsoft.VSTS.Feedback Fields
	WorkItemFieldApplicationLaunchInstructions AzureWorkItemField = "Microsoft.VSTS.Feedback.ApplicationLaunchInstructions"
	WorkItemFieldApplicationStartInformation   AzureWorkItemField = "Microsoft.VSTS.Feedback.ApplicationStartInformation"
	WorkItemFieldApplicationType               AzureWorkItemField = "Microsoft.VSTS.Feedback.ApplicationType"

	// Microsoft.VSTS.TCM (Test Case Management) Fields
	WorkItemFieldAutomatedTestId      AzureWorkItemField = "Microsoft.VSTS.TCM.AutomatedTestId"
	WorkItemFieldAutomatedTestName    AzureWorkItemField = "Microsoft.VSTS.TCM.AutomatedTestName"
	WorkItemFieldAutomatedTestStorage AzureWorkItemField = "Microsoft.VSTS.TCM.AutomatedTestStorage"
	WorkItemFieldAutomatedTestType    AzureWorkItemField = "Microsoft.VSTS.TCM.AutomatedTestType"
	WorkItemFieldAutomationStatus     AzureWorkItemField = "Microsoft.VSTS.TCM.AutomationStatus"
	WorkItemFieldLocalDataSource      AzureWorkItemField = "Microsoft.VSTS.TCM.LocalDataSource"
	WorkItemFieldParameters           AzureWorkItemField = "Microsoft.VSTS.TCM.Parameters"
	WorkItemFieldQueryText            AzureWorkItemField = "Microsoft.VSTS.TCM.QueryText"
	WorkItemFieldReproSteps           AzureWorkItemField = "Microsoft.VSTS.TCM.ReproSteps"
	WorkItemFieldSteps                AzureWorkItemField = "Microsoft.VSTS.TCM.Steps"
	WorkItemFieldSystemInfo           AzureWorkItemField = "Microsoft.VSTS.TCM.SystemInfo"
	WorkItemFieldTestSuiteAudit       AzureWorkItemField = "Microsoft.VSTS.TCM.TestSuiteAudit"
	WorkItemFieldTestSuiteType        AzureWorkItemField = "Microsoft.VSTS.TCM.TestSuiteType"
	WorkItemFieldTestSuiteTypeId      AzureWorkItemField = "Microsoft.VSTS.TCM.TestSuiteTypeId"
)

// Common field groups for easier usage
var (
	// Core system fields that are typically needed for basic work item operations
	CoreSystemFields = []AzureWorkItemField{
		WorkItemFieldId,
		WorkItemFieldTitle,
		WorkItemFieldDescription,
		WorkItemFieldState,
		WorkItemFieldWorkItemType,
		WorkItemFieldAssignedTo,
		WorkItemFieldAreaPath,
		WorkItemFieldIterationPath,
		WorkItemFieldCreatedBy,
		WorkItemFieldCreatedDate,
		WorkItemFieldChangedBy,
		WorkItemFieldChangedDate,
	}

	// Priority and severity related fields
	PriorityFields = []AzureWorkItemField{
		WorkItemFieldPriority,
		WorkItemFieldSeverity,
		WorkItemFieldRisk,
		WorkItemFieldBusinessValue,
	}

	// Scheduling and effort tracking fields
	SchedulingFields = []AzureWorkItemField{
		WorkItemFieldOriginalEstimate,
		WorkItemFieldRemainingWork,
		WorkItemFieldCompletedWork,
		WorkItemFieldStoryPoints,
		WorkItemFieldEffort,
		WorkItemFieldStartDate,
		WorkItemFieldFinishDate,
		WorkItemFieldDueDate,
		WorkItemFieldTargetDate,
	}

	// State transition related fields
	StateFields = []AzureWorkItemField{
		WorkItemFieldState,
		WorkItemFieldReason,
		WorkItemFieldStateCode,
		WorkItemFieldStateChangeDate,
		WorkItemFieldActivatedBy,
		WorkItemFieldActivatedDate,
		WorkItemFieldResolvedBy,
		WorkItemFieldResolvedDate,
		WorkItemFieldResolvedReason,
		WorkItemFieldClosedBy,
		WorkItemFieldClosedDate,
	}

	// Test case management fields
	TestFields = []AzureWorkItemField{
		WorkItemFieldAutomatedTestId,
		WorkItemFieldAutomatedTestName,
		WorkItemFieldAutomatedTestStorage,
		WorkItemFieldAutomatedTestType,
		WorkItemFieldAutomationStatus,
		WorkItemFieldReproSteps,
		WorkItemFieldSteps,
		WorkItemFieldSystemInfo,
	}
)

// AllWorkItemFields contains every declared field constant. It's used to build
// a reverse lookup map so callers can parse a string into a known AzureWorkItemField.
var AllWorkItemFields = []AzureWorkItemField{
	// System fields
	WorkItemFieldTitle,
	WorkItemFieldDescription,
	WorkItemFieldState,
	WorkItemFieldAreaId,
	WorkItemFieldAreaPath,
	WorkItemFieldAssignedTo,
	WorkItemFieldAttachedFileCount,
	WorkItemFieldAuthorizedAs,
	WorkItemFieldAuthorizedDate,
	WorkItemFieldBoardColumn,
	WorkItemFieldBoardColumnDone,
	WorkItemFieldBoardLane,
	WorkItemFieldChangedBy,
	WorkItemFieldChangedDate,
	WorkItemFieldCommentCount,
	WorkItemFieldCreatedBy,
	WorkItemFieldCreatedDate,
	WorkItemFieldExternalLinkCount,
	WorkItemFieldHistory,
	WorkItemFieldHyperLinkCount,
	WorkItemFieldId,
	WorkItemFieldIterationId,
	WorkItemFieldIterationPath,
	WorkItemFieldNodeName,
	WorkItemFieldParent,
	WorkItemFieldReason,
	WorkItemFieldRelatedLinkCount,
	WorkItemFieldRemoteLinkCount,
	WorkItemFieldRev,
	WorkItemFieldRevisedDate,
	WorkItemFieldTags,
	WorkItemFieldTeamProject,
	WorkItemFieldWatermark,
	WorkItemFieldWorkItemType,

	// Microsoft.VSTS.Common
	WorkItemFieldAcceptanceCriteria,
	WorkItemFieldActivatedBy,
	WorkItemFieldActivatedDate,
	WorkItemFieldActivity,
	WorkItemFieldBusinessValue,
	WorkItemFieldClosedBy,
	WorkItemFieldClosedDate,
	WorkItemFieldIssue,
	WorkItemFieldPriority,
	WorkItemFieldRating,
	WorkItemFieldResolvedBy,
	WorkItemFieldResolvedDate,
	WorkItemFieldResolvedReason,
	WorkItemFieldReviewedBy,
	WorkItemFieldRisk,
	WorkItemFieldSeverity,
	WorkItemFieldStackRank,
	WorkItemFieldStateChangeDate,
	WorkItemFieldStateCode,
	WorkItemFieldTimeCriticality,
	WorkItemFieldValueArea,

	// Microsoft.VSTS.Scheduling
	WorkItemFieldCompletedWork,
	WorkItemFieldDueDate,
	WorkItemFieldEffort,
	WorkItemFieldFinishDate,
	WorkItemFieldOriginalEstimate,
	WorkItemFieldRemainingWork,
	WorkItemFieldStartDate,
	WorkItemFieldStoryPoints,
	WorkItemFieldTargetDate,

	// Build
	WorkItemFieldFoundIn,
	WorkItemFieldIntegrationBuild,

	// CodeReview
	WorkItemFieldAcceptedBy,
	WorkItemFieldAcceptedDate,
	WorkItemFieldClosedStatus,
	WorkItemFieldClosedStatusCode,
	WorkItemFieldClosingComment,
	WorkItemFieldContext,
	WorkItemFieldContextCode,
	WorkItemFieldContextOwner,
	WorkItemFieldContextType,

	// Feedback
	WorkItemFieldApplicationLaunchInstructions,
	WorkItemFieldApplicationStartInformation,
	WorkItemFieldApplicationType,

	// TCM
	WorkItemFieldAutomatedTestId,
	WorkItemFieldAutomatedTestName,
	WorkItemFieldAutomatedTestStorage,
	WorkItemFieldAutomatedTestType,
	WorkItemFieldAutomationStatus,
	WorkItemFieldLocalDataSource,
	WorkItemFieldParameters,
	WorkItemFieldQueryText,
	WorkItemFieldReproSteps,
	WorkItemFieldSteps,
	WorkItemFieldSystemInfo,
	WorkItemFieldTestSuiteAudit,
	WorkItemFieldTestSuiteType,
	WorkItemFieldTestSuiteTypeId,
}

var workItemFieldLookup map[string]AzureWorkItemField

func init() {
	workItemFieldLookup = make(map[string]AzureWorkItemField, len(AllWorkItemFields))
	for _, f := range AllWorkItemFields {
		workItemFieldLookup[string(f)] = f
	}
}

// ParseAzureWorkItemField returns a pointer to the matching AzureWorkItemField for
// the given string. The input may be provided as either the raw reference name
// (e.g. "System.Title") or the JSON patch path (e.g. "/fields/System.Title").
// If the field is not known, it returns nil.
func ParseAzureWorkItemField(s string) *AzureWorkItemField {
	// Accept optional "/fields/" prefix
	s = strings.TrimPrefix(s, "/fields/")
	if v, ok := workItemFieldLookup[s]; ok {
		vv := v
		return &vv
	}
	return nil
}
