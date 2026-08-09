package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	providerops "ecommerce/internal/services/providerops"
	"ecommerce/models"
)

func (e *CheckoutProviderEndpoints) ListAdminProviderCredentials(ctx context.Context, r apicontract.ListAdminProviderCredentialsRequestObject) (apicontract.ListAdminProviderCredentialsResponseObject, error) {
	values, err := e.runtime.Credentials.List(ctx, e.db, enumString(r.Params.ProviderType))
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.ProviderCredential, 0, len(values))
	for _, v := range values {
		data = append(data, credentialContract(v))
	}
	return apicontract.ListAdminProviderCredentials200JSONResponse{Data: data}, nil
}
func (e *CheckoutProviderEndpoints) UpsertAdminProviderCredential(ctx context.Context, r apicontract.UpsertAdminProviderCredentialRequestObject) (apicontract.UpsertAdminProviderCredentialResponseObject, error) {
	if r.Body == nil {
		return nil, errors.New("provider credential body is required")
	}
	label, settlement, fx := "", "", ""
	currencies := []string{}
	if r.Body.Label != nil {
		label = *r.Body.Label
	}
	if r.Body.SettlementCurrency != nil {
		settlement = *r.Body.SettlementCurrency
	}
	if r.Body.FxMode != nil {
		fx = string(*r.Body.FxMode)
	}
	if r.Body.SupportedCurrencies != nil {
		currencies = *r.Body.SupportedCurrencies
	}
	v, err := e.runtime.Credentials.Store(ctx, e.db, providerops.StoreCredentialInput{ProviderType: string(r.Body.ProviderType), ProviderID: r.Body.ProviderId, Environment: string(r.Body.Environment), Label: label, SecretData: r.Body.SecretData, Metadata: providerops.CredentialMetadata{SupportedCurrencies: currencies, SettlementCurrency: settlement, FXMode: fx}})
	if err != nil {
		return nil, err
	}
	return apicontract.UpsertAdminProviderCredential200JSONResponse{Credential: credentialContract(v)}, nil
}
func (e *CheckoutProviderEndpoints) RotateAdminProviderCredential(ctx context.Context, r apicontract.RotateAdminProviderCredentialRequestObject) (apicontract.RotateAdminProviderCredentialResponseObject, error) {
	v, err := e.runtime.Credentials.Rotate(ctx, e.db, uint(r.Id))
	if err != nil {
		return nil, err
	}
	return apicontract.RotateAdminProviderCredential200JSONResponse{Credential: credentialContract(v)}, nil
}
func credentialContract(v providerops.StoredCredential) apicontract.ProviderCredential {
	var settlement *string
	if strings.TrimSpace(v.Metadata.SettlementCurrency) != "" {
		x := v.Metadata.SettlementCurrency
		settlement = &x
	}
	return apicontract.ProviderCredential{Id: int(v.Record.ID), ProviderType: apicontract.ProviderCredentialProviderType(v.Record.ProviderType), ProviderId: v.Record.ProviderID, Environment: apicontract.ProviderCredentialEnvironment(v.Record.Environment), Label: v.Record.Label, KeyVersion: v.Record.KeyVersion, SupportedCurrencies: append([]string(nil), v.Metadata.SupportedCurrencies...), SettlementCurrency: settlement, FxMode: apicontract.ProviderCredentialFxMode(v.Metadata.FXMode), LastRotatedAt: v.Record.LastRotatedAt, UpdatedAt: v.Record.UpdatedAt}
}

func (e *CheckoutProviderEndpoints) ListAdminProviderOperations(ctx context.Context, r apicontract.ListAdminProviderOperationsRequestObject) (apicontract.ListAdminProviderOperationsResponseObject, error) {
	page, limit := intPtr(r.Params.Page), intPtr(r.Params.Limit)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	entityID := uint(0)
	if r.Params.EntityId != nil && *r.Params.EntityId > 0 {
		entityID = uint(*r.Params.EntityId)
	}
	values, total, err := e.queries.ListOperations(ctx, providerops.ListOperationsInput{ProviderType: enumString(r.Params.ProviderType), ProviderID: stringPtr(r.Params.ProviderId), Environment: enumString(r.Params.Environment), Operation: stringPtr(r.Params.Operation), Status: enumString(r.Params.Status), EntityType: stringPtr(r.Params.EntityType), EntityID: entityID, Page: page, Limit: limit})
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.ProviderOperation, 0, len(values))
	for _, v := range values {
		data = append(data, providerOperationContract(v, nil, e.admin.AvailableActions(v)))
	}
	return apicontract.ListAdminProviderOperations200JSONResponse{Data: data, Pagination: pagination(page, limit, total)}, nil
}
func (e *CheckoutProviderEndpoints) GetAdminProviderOperation(ctx context.Context, r apicontract.GetAdminProviderOperationRequestObject) (apicontract.GetAdminProviderOperationResponseObject, error) {
	v, err := e.queries.GetOperation(ctx, uint(r.Id))
	if err != nil {
		return nil, err
	}
	child, err := e.queries.CompensationOperationID(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	return apicontract.GetAdminProviderOperation200JSONResponse{Operation: providerOperationContract(v, child, e.admin.AvailableActions(v))}, nil
}
func (e *CheckoutProviderEndpoints) QueryAdminProviderOperationOutcome(ctx context.Context, r apicontract.QueryAdminProviderOperationOutcomeRequestObject) (apicontract.QueryAdminProviderOperationOutcomeResponseObject, error) {
	v, err := e.admin.QueryOutcome(ctx, uint(r.Id))
	envelope := apicontract.ProviderOperationEnvelope{Operation: providerOperationContract(v, nil, e.admin.AvailableActions(v))}
	if errors.Is(err, providerops.ErrProviderOperationCapabilityBlocked) {
		problem := providerAdminProblem(ctx, e.renderer, http.StatusPreconditionFailed, "provider_capability_unavailable", err.Error(), err)
		return apicontract.QueryAdminProviderOperationOutcome412ApplicationProblemPlusJSONResponse{PreconditionFailedProblemApplicationProblemPlusJSONResponse: apicontract.PreconditionFailedProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if errors.Is(err, providerops.ErrOutcomeUnknown) || (err != nil && v.ID != 0 && v.Status == models.ProviderOperationStatusReconciliationRequired) {
		return apicontract.QueryAdminProviderOperationOutcome202JSONResponse(envelope), nil
	}
	if err != nil {
		return nil, err
	}
	return apicontract.QueryAdminProviderOperationOutcome200JSONResponse(envelope), nil
}
func (e *CheckoutProviderEndpoints) RetryFinalizeAdminProviderOperation(ctx context.Context, r apicontract.RetryFinalizeAdminProviderOperationRequestObject) (apicontract.RetryFinalizeAdminProviderOperationResponseObject, error) {
	v, err := e.admin.RetryFinalize(ctx, uint(r.Id))
	if errors.Is(err, providerops.ErrProviderOperationActionConflict) {
		problem := providerAdminProblem(ctx, e.renderer, http.StatusConflict, "provider_operation_state_conflict", err.Error(), err)
		return apicontract.RetryFinalizeAdminProviderOperation409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err != nil {
		return nil, err
	}
	return apicontract.RetryFinalizeAdminProviderOperation202JSONResponse{Operation: providerOperationContract(v, nil, e.admin.AvailableActions(v))}, nil
}
func (e *CheckoutProviderEndpoints) RetryCompensationAdminProviderOperation(ctx context.Context, r apicontract.RetryCompensationAdminProviderOperationRequestObject) (apicontract.RetryCompensationAdminProviderOperationResponseObject, error) {
	v, err := e.admin.RetryCompensation(ctx, uint(r.Id))
	if errors.Is(err, providerops.ErrProviderOperationActionConflict) {
		problem := providerAdminProblem(ctx, e.renderer, http.StatusConflict, "provider_operation_state_conflict", err.Error(), err)
		return apicontract.RetryCompensationAdminProviderOperation409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err != nil {
		return nil, err
	}
	return apicontract.RetryCompensationAdminProviderOperation202JSONResponse{Operation: providerOperationContract(v, nil, e.admin.AvailableActions(v))}, nil
}

func (e *CheckoutProviderEndpoints) GetAdminProviderOperationsOverview(ctx context.Context, _ apicontract.GetAdminProviderOperationsOverviewRequestObject) (apicontract.GetAdminProviderOperationsOverviewResponseObject, error) {
	v, err := e.overview.Get(ctx)
	if err != nil {
		return nil, err
	}
	return apicontract.GetAdminProviderOperationsOverview200JSONResponse{RuntimeEnvironment: apicontract.ProviderOperationsOverviewRuntimeEnvironment(v.RuntimeEnvironment), CredentialServiceConfigured: v.CredentialServiceConfigured, WebhookEvents: apicontract.ProviderWebhookStatusSummary{PendingCount: int(v.WebhookEvents.PendingCount), ProcessedCount: int(v.WebhookEvents.ProcessedCount), DeadLetterCount: int(v.WebhookEvents.DeadLetterCount), RejectedCount: int(v.WebhookEvents.RejectedCount)}, Operations: apicontract.ProviderOperationStatusSummary{TotalCount: int(v.Operations.TotalCount), ActiveCount: int(v.Operations.ActiveCount), UnknownCount: int(v.Operations.UnknownCount), FinalizeRetryCount: int(v.Operations.FinalizeRetryCount), CompensationRetryCount: int(v.Operations.CompensationRetryCount), FailedCount: int(v.Operations.FailedCount), CompletedCount: int(v.Operations.CompletedCount)}, ReconciliationCases: apicontract.ProviderReconciliationCaseSummary{OpenCount: int(v.ReconciliationCases.OpenCount), UnassignedCount: int(v.ReconciliationCases.UnassignedCount)}}, nil
}

func (e *CheckoutProviderEndpoints) ListAdminProviderReconciliationCases(ctx context.Context, r apicontract.ListAdminProviderReconciliationCasesRequestObject) (apicontract.ListAdminProviderReconciliationCasesResponseObject, error) {
	page, limit := intPtr(r.Params.Page), intPtr(r.Params.Limit)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	values, total, err := e.cases.List(ctx, providerops.ListReconciliationCasesInput{Status: enumString(r.Params.Status), ProviderType: enumString(r.Params.ProviderType), ProviderID: stringPtr(r.Params.ProviderId), CaseType: stringPtr(r.Params.CaseType), Page: page, Limit: limit})
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.ProviderReconciliationCase, 0, len(values))
	for _, v := range values {
		data = append(data, reconciliationCaseContract(v))
	}
	return apicontract.ListAdminProviderReconciliationCases200JSONResponse{Data: data, Pagination: pagination(page, limit, total)}, nil
}
func (e *CheckoutProviderEndpoints) GetAdminProviderReconciliationCase(ctx context.Context, r apicontract.GetAdminProviderReconciliationCaseRequestObject) (apicontract.GetAdminProviderReconciliationCaseResponseObject, error) {
	v, err := e.cases.Get(ctx, uint(r.Id))
	if errors.Is(err, providerops.ErrProviderReconciliationCaseNotFound) {
		problem := providerAdminProblem(ctx, e.renderer, http.StatusNotFound, "provider_reconciliation_case_not_found", err.Error(), err)
		return apicontract.GetAdminProviderReconciliationCase404ApplicationProblemPlusJSONResponse{NotFoundProblemApplicationProblemPlusJSONResponse: apicontract.NotFoundProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err != nil {
		return nil, err
	}
	return apicontract.GetAdminProviderReconciliationCase200JSONResponse{Case: reconciliationCaseContract(v)}, nil
}
func (e *CheckoutProviderEndpoints) UpdateAdminProviderReconciliationCase(ctx context.Context, r apicontract.UpdateAdminProviderReconciliationCaseRequestObject) (apicontract.UpdateAdminProviderReconciliationCaseResponseObject, error) {
	if r.Body == nil {
		return nil, errors.New("reconciliation case body is required")
	}
	var status, outcome *string
	if r.Body.Status != nil {
		x := string(*r.Body.Status)
		status = &x
	}
	if r.Body.Outcome != nil {
		x := string(*r.Body.Outcome)
		outcome = &x
	}
	v, err := e.cases.Update(ctx, uint(r.Id), providerops.UpdateReconciliationCaseInput{AssignedTo: r.Body.AssignedTo, Status: status, Outcome: outcome, ResolutionNote: r.Body.ResolutionNote})
	switch {
	case errors.Is(err, providerops.ErrInvalidProviderReconciliationCaseUpdate):
		problem := providerAdminProblem(ctx, e.renderer, http.StatusBadRequest, "invalid_provider_reconciliation_case_update", err.Error(), err)
		return apicontract.UpdateAdminProviderReconciliationCase400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	case errors.Is(err, providerops.ErrProviderReconciliationCaseNotFound):
		problem := providerAdminProblem(ctx, e.renderer, http.StatusNotFound, "provider_reconciliation_case_not_found", err.Error(), err)
		return apicontract.UpdateAdminProviderReconciliationCase404ApplicationProblemPlusJSONResponse{NotFoundProblemApplicationProblemPlusJSONResponse: apicontract.NotFoundProblemApplicationProblemPlusJSONResponse(problem)}, nil
	case errors.Is(err, providerops.ErrProviderReconciliationCaseConflict):
		problem := providerAdminProblem(ctx, e.renderer, http.StatusConflict, "provider_reconciliation_case_conflict", err.Error(), err)
		return apicontract.UpdateAdminProviderReconciliationCase409ApplicationProblemPlusJSONResponse{ConflictProblemApplicationProblemPlusJSONResponse: apicontract.ConflictProblemApplicationProblemPlusJSONResponse(problem)}, nil
	case err != nil:
		return nil, err
	}
	return apicontract.UpdateAdminProviderReconciliationCase200JSONResponse{Case: reconciliationCaseContract(v)}, nil
}

func (e *CheckoutProviderEndpoints) ListAdminProviderReconciliationRuns(ctx context.Context, r apicontract.ListAdminProviderReconciliationRunsRequestObject) (apicontract.ListAdminProviderReconciliationRunsResponseObject, error) {
	page, limit := intPtr(r.Params.Page), intPtr(r.Params.Limit)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	values, total, err := e.runtime.Reconciliation.ListRuns(ctx, enumString(r.Params.ProviderType), stringPtr(r.Params.ProviderId), page, limit)
	if err != nil {
		return nil, err
	}
	data := make([]apicontract.ProviderReconciliationRun, 0, len(values))
	for _, v := range values {
		data = append(data, reconciliationRunContract(v, false))
	}
	return apicontract.ListAdminProviderReconciliationRuns200JSONResponse{Data: data, Pagination: pagination(page, limit, total)}, nil
}
func (e *CheckoutProviderEndpoints) CreateAdminProviderReconciliationRun(ctx context.Context, r apicontract.CreateAdminProviderReconciliationRunRequestObject) (apicontract.CreateAdminProviderReconciliationRunResponseObject, error) {
	if r.Body == nil {
		return nil, errors.New("reconciliation run body is required")
	}
	v, _, err := e.runtime.Reconciliation.Run(ctx, providerops.ReconciliationRunInput{ProviderType: string(r.Body.ProviderType), ProviderID: r.Body.ProviderId, Trigger: models.ProviderReconciliationTriggerManual})
	if err != nil {
		return nil, err
	}
	return apicontract.CreateAdminProviderReconciliationRun201JSONResponse{Run: reconciliationRunContract(v, true)}, nil
}
func (e *CheckoutProviderEndpoints) GetAdminProviderReconciliationRun(ctx context.Context, r apicontract.GetAdminProviderReconciliationRunRequestObject) (apicontract.GetAdminProviderReconciliationRunResponseObject, error) {
	v, err := e.runtime.Reconciliation.GetRun(ctx, uint(r.Id))
	if err != nil {
		return nil, err
	}
	return apicontract.GetAdminProviderReconciliationRun200JSONResponse{Run: reconciliationRunContract(v, true)}, nil
}

func providerAdminProblem(ctx context.Context, renderer Renderer, status int, code, detail string, cause error) apicontract.Problem {
	problem := renderer.FromError(ctx, status, ErrorProblem(Problem{Status: status, Code: code, Detail: detail}, cause))
	return apicontract.Problem{Type: problem.Type, Title: problem.Title, Status: int32(problem.Status), Detail: problem.Detail, Code: problem.Code, CorrelationId: problem.CorrelationID}
}

func pagination(page, limit int, total int64) apicontract.Pagination {
	pages := int(total) / limit
	if int(total)%limit != 0 {
		pages++
	}
	return apicontract.Pagination{Page: page, Limit: limit, Total: int(total), TotalPages: pages}
}
func providerOperationContract(v models.ProviderOperation, compensationID *uint, actions []string) apicontract.ProviderOperation {
	available := make([]apicontract.ProviderOperationAvailableActions, 0, len(actions))
	for _, a := range actions {
		available = append(available, apicontract.ProviderOperationAvailableActions(a))
	}
	out := apicontract.ProviderOperation{Id: int(v.ID), OperationKey: v.OperationKey, ProviderType: apicontract.ProviderOperationProviderType(v.ProviderType), ProviderId: v.ProviderID, Environment: apicontract.ProviderOperationEnvironment(v.Environment), Operation: v.Operation, IdempotencyKey: v.IdempotencyKey, CorrelationId: v.CorrelationID, EntityType: v.EntityType, EntityId: int(v.EntityID), Status: apicontract.ProviderOperationStatus(v.Status), Retryable: len(actions) > 0, AvailableActions: &available, Attempts: []apicontract.ProviderOperationAttempt{}, ReconciliationCases: []apicontract.ProviderReconciliationCase{}, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, CompletedAt: v.CompletedAt, NextAttemptAt: v.NextAttemptAt}
	if v.ParentOperationID != nil {
		x := int(*v.ParentOperationID)
		out.ParentOperationId = &x
	}
	if compensationID != nil {
		x := int(*compensationID)
		out.CompensationOperationId = &x
	}
	if v.ProviderOutcome != "" {
		out.ProviderOutcome = &v.ProviderOutcome
	}
	if v.ProviderReference != "" {
		out.ProviderReference = &v.ProviderReference
	}
	for _, a := range v.Attempts {
		attempt := apicontract.ProviderOperationAttempt{Id: int(a.ID), AttemptNumber: a.AttemptNumber, Phase: a.Phase, Outcome: apicontract.ProviderOperationAttemptOutcome(a.Outcome), ProviderOutcome: apicontract.ProviderOperationAttemptProviderOutcome(a.ProviderOutcome), OperationKey: a.OperationKey, Retryable: a.Retryable, StartedAt: a.StartedAt, FinishedAt: a.FinishedAt}
		if a.ProviderReference != "" {
			attempt.ProviderReference = &a.ProviderReference
		}
		if a.ErrorMessage != "" {
			attempt.Problem = &apicontract.Problem{
				Type:          TypeInternal,
				Title:         "Provider Operation Failed",
				Status:        http.StatusBadGateway,
				Detail:        a.ErrorMessage,
				Code:          "provider_operation_failed",
				CorrelationId: v.CorrelationID,
			}
		}
		out.Attempts = append(out.Attempts, attempt)
	}
	for _, c := range v.ReconciliationCases {
		out.ReconciliationCases = append(out.ReconciliationCases, reconciliationCaseContract(providerops.DescribeReconciliationCase(c, v)))
	}
	return out
}
func reconciliationCaseContract(v providerops.ReconciliationCaseRecord) apicontract.ProviderReconciliationCase {
	out := apicontract.ProviderReconciliationCase{Id: int(v.Case.ID), OperationId: int(v.Case.ProviderOperationID), ProviderType: apicontract.ProviderReconciliationCaseProviderType(v.Operation.ProviderType), ProviderId: v.Operation.ProviderID, Environment: apicontract.ProviderReconciliationCaseEnvironment(v.Operation.Environment), Operation: v.Operation.Operation, OperationStatus: apicontract.ProviderOperationStatus(v.Operation.Status), Status: apicontract.ProviderReconciliationCaseStatus(v.Case.Status), Reason: v.Case.Reason, CaseType: v.Case.CaseType, ProviderOutcome: apicontract.ProviderReconciliationCaseProviderOutcome(v.Case.ProviderOutcome), OperationKey: v.Case.OperationKey, NextAttemptAt: v.Case.NextAttemptAt, OpenedAt: v.Case.OpenedAt, ResolvedAt: v.Case.ResolvedAt}
	if v.Case.AttemptID != nil {
		x := int(*v.Case.AttemptID)
		out.AttemptId = &x
	}
	if v.Case.Outcome != "" {
		x := apicontract.ProviderReconciliationCaseOutcome(v.Case.Outcome)
		out.Outcome = &x
	}
	if v.AssignedTo != "" {
		out.AssignedTo = &v.AssignedTo
	}
	if v.ResolutionNote != "" {
		out.ResolutionNote = &v.ResolutionNote
	}
	return out
}
func reconciliationRunContract(v models.ProviderReconciliationRun, include bool) apicontract.ProviderReconciliationRun {
	out := apicontract.ProviderReconciliationRun{Id: int(v.ID), ProviderType: apicontract.ProviderReconciliationRunProviderType(v.ProviderType), ProviderId: v.ProviderID, Environment: apicontract.ProviderReconciliationRunEnvironment(v.Environment), Trigger: apicontract.ProviderReconciliationRunTrigger(v.Trigger), Status: apicontract.ProviderReconciliationRunStatus(v.Status), CheckedCount: v.CheckedCount, DriftCount: v.DriftCount, ErrorCount: v.ErrorCount, StartedAt: v.StartedAt, FinishedAt: v.FinishedAt}
	if include {
		drifts := make([]apicontract.ProviderReconciliationDrift, 0, len(v.Drifts))
		for _, d := range v.Drifts {
			drifts = append(drifts, apicontract.ProviderReconciliationDrift{Id: int(d.ID), EntityType: d.EntityType, EntityId: int(d.EntityID), ProviderReference: d.ProviderReference, Severity: apicontract.ProviderReconciliationDriftSeverity(d.Severity), FieldName: d.FieldName, ExpectedValue: d.ExpectedValue, ActualValue: d.ActualValue, Message: d.Message})
		}
		out.Drifts = &drifts
	}
	return out
}

func strictProviderProblem(status int, code, detail string) error {
	return problemError(status, code, detail, errors.New(detail))
}

var _ = http.StatusAccepted
