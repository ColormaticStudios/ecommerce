package httpapi

import (
	"context"
	"fmt"
	"net/http"

	"ecommerce/internal/apicontract"
	searchservice "ecommerce/internal/search"
)

func (e *CatalogEndpoints) ListAdminSearchMerchandisingRules(ctx context.Context, _ apicontract.ListAdminSearchMerchandisingRulesRequestObject) (apicontract.ListAdminSearchMerchandisingRulesResponseObject, error) {
	values, err := e.search.ListMerchandisingRules(ctx)
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	data := make([]apicontract.SearchMerchandisingRule, 0, len(values))
	for _, value := range values {
		data = append(data, merchandisingRuleContract(value))
	}
	return apicontract.ListAdminSearchMerchandisingRules200JSONResponse{Data: data}, nil
}
func (e *CatalogEndpoints) CreateAdminSearchMerchandisingRule(ctx context.Context, request apicontract.CreateAdminSearchMerchandisingRuleRequestObject) (apicontract.CreateAdminSearchMerchandisingRuleResponseObject, error) {
	if request.Body == nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "A merchandising rule body is required.", nil)
	}
	input, err := merchandisingRuleInput(*request.Body)
	if err != nil {
		return nil, err
	}
	value, err := e.search.CreateMerchandisingRule(ctx, input, searchConfigurationActorID(ctx))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.CreateAdminSearchMerchandisingRule201JSONResponse(merchandisingRuleContract(value)), nil
}
func (e *CatalogEndpoints) GetAdminSearchMerchandisingRule(ctx context.Context, request apicontract.GetAdminSearchMerchandisingRuleRequestObject) (apicontract.GetAdminSearchMerchandisingRuleResponseObject, error) {
	if request.Id < 1 {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Merchandising rule ID must be positive.", nil)
	}
	value, err := e.search.GetMerchandisingRule(ctx, uint(request.Id))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.GetAdminSearchMerchandisingRule200JSONResponse(merchandisingRuleContract(value)), nil
}
func (e *CatalogEndpoints) UpdateAdminSearchMerchandisingRule(ctx context.Context, request apicontract.UpdateAdminSearchMerchandisingRuleRequestObject) (apicontract.UpdateAdminSearchMerchandisingRuleResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "A valid merchandising rule ID and body are required.", nil)
	}
	body := request.Body
	input := searchservice.MerchandisingRulePatch{Name: body.Name, Priority: body.Priority, IsActive: body.IsActive, StartsAt: body.StartsAt, EndsAt: body.EndsAt}
	if body.RuleType != nil {
		value := string(*body.RuleType)
		input.RuleType = &value
	}
	if body.Predicate != nil {
		value := merchandisingPredicateInput(*body.Predicate)
		input.Predicate = &value
	}
	if body.Action != nil {
		value, err := merchandisingActionInput(*body.Action)
		if err != nil {
			return nil, err
		}
		input.Action = &value
	}
	if body.ClearStartsAt != nil {
		input.ClearStartsAt = *body.ClearStartsAt
	}
	if body.ClearEndsAt != nil {
		input.ClearEndsAt = *body.ClearEndsAt
	}
	value, err := e.search.UpdateMerchandisingRule(ctx, uint(request.Id), input, searchConfigurationActorID(ctx))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.UpdateAdminSearchMerchandisingRule200JSONResponse(merchandisingRuleContract(value)), nil
}
func (e *CatalogEndpoints) DeleteAdminSearchMerchandisingRule(ctx context.Context, request apicontract.DeleteAdminSearchMerchandisingRuleRequestObject) (apicontract.DeleteAdminSearchMerchandisingRuleResponseObject, error) {
	if request.Id < 1 {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Merchandising rule ID must be positive.", nil)
	}
	if err := e.search.DeleteMerchandisingRule(ctx, uint(request.Id), searchConfigurationActorID(ctx)); err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.DeleteAdminSearchMerchandisingRule204Response{}, nil
}
func (e *CatalogEndpoints) ListAdminSearchMerchandisingRuleAudit(ctx context.Context, request apicontract.ListAdminSearchMerchandisingRuleAuditRequestObject) (apicontract.ListAdminSearchMerchandisingRuleAuditResponseObject, error) {
	if request.Id < 1 {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Merchandising rule ID must be positive.", nil)
	}
	values, err := e.search.ListMerchandisingAudit(ctx, uint(request.Id))
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.ListAdminSearchMerchandisingRuleAudit200JSONResponse{Data: merchandisingAuditContract(values)}, nil
}

func (e *CatalogEndpoints) ListAdminSearchMerchandisingAudit(ctx context.Context, _ apicontract.ListAdminSearchMerchandisingAuditRequestObject) (apicontract.ListAdminSearchMerchandisingAuditResponseObject, error) {
	values, err := e.search.ListMerchandisingAudit(ctx, 0)
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	return apicontract.ListAdminSearchMerchandisingAudit200JSONResponse{Data: merchandisingAuditContract(values)}, nil
}

func merchandisingAuditContract(values []searchservice.MerchandisingAudit) []apicontract.SearchMerchandisingAudit {
	data := make([]apicontract.SearchMerchandisingAudit, 0, len(values))
	for _, value := range values {
		entry := apicontract.SearchMerchandisingAudit{Id: int(value.ID), RuleId: int(value.RuleID), Operation: apicontract.SearchMerchandisingAuditOperation(value.Operation), ActorId: searchConfigurationUpdatedBy(value.ActorID), CreatedAt: value.CreatedAt}
		if value.Before != nil {
			snapshot := merchandisingRuleContract(*value.Before)
			entry.Before = &snapshot
		}
		if value.After != nil {
			snapshot := merchandisingRuleContract(*value.After)
			entry.After = &snapshot
		}
		data = append(data, entry)
	}
	return data
}

func (e *CatalogEndpoints) PreviewAdminSearch(ctx context.Context, request apicontract.PreviewAdminSearchRequestObject) (apicontract.PreviewAdminSearchResponseObject, error) {
	if request.Body == nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "A search preview body is required.", nil)
	}
	if request.Body.Filters == nil {
		return nil, problemError(http.StatusBadRequest, "invalid_request", "Search preview filters are required.", nil)
	}
	params := previewSearchParams(*request.Body.Filters)
	filters, err := searchProductFilters(params, true)
	if err != nil {
		return nil, err
	}
	input := searchservice.MerchandisingPreviewInput{Filters: filters, At: request.Body.At}
	if request.Body.Rules != nil {
		rules := make([]searchservice.MerchandisingRule, 0, len(*request.Body.Rules))
		for _, rule := range *request.Body.Rules {
			body := apicontract.SearchMerchandisingRuleInput{Name: rule.Name, RuleType: apicontract.SearchMerchandisingRuleInputRuleType(rule.RuleType), Predicate: rule.Predicate, Action: rule.Action, Priority: rule.Priority, StartsAt: rule.StartsAt, EndsAt: rule.EndsAt, IsActive: rule.IsActive}
			value, err := merchandisingRuleInput(body)
			if err != nil {
				return nil, err
			}
			id := uint(0)
			if rule.Id != nil {
				if *rule.Id < 0 {
					return nil, problemError(http.StatusBadRequest, "invalid_request", "Preview rule IDs cannot be negative.", nil)
				}
				id = uint(*rule.Id)
			}
			rules = append(rules, searchservice.MerchandisingRule{MerchandisingRuleInput: value, ID: id})
		}
		input.Rules = &rules
	}
	result, err := e.search.PreviewMerchandising(ctx, input)
	if err != nil {
		return nil, searchConfigurationEndpointError(err)
	}
	baseline, err := e.searchResultContract(ctx, result.Baseline, filters)
	if err != nil {
		return nil, err
	}
	proposed, err := e.searchResultContract(ctx, result.Proposed, filters)
	if err != nil {
		return nil, err
	}
	return apicontract.PreviewAdminSearch200JSONResponse{Baseline: baseline, Proposed: proposed}, nil
}

func merchandisingRuleInput(body apicontract.SearchMerchandisingRuleInput) (searchservice.MerchandisingRuleInput, error) {
	action, err := merchandisingActionInput(body.Action)
	if err != nil {
		return searchservice.MerchandisingRuleInput{}, err
	}
	input := searchservice.MerchandisingRuleInput{Name: body.Name, RuleType: string(body.RuleType), Action: action, StartsAt: body.StartsAt, EndsAt: body.EndsAt, IsActive: true}
	if body.Predicate != nil {
		input.Predicate = merchandisingPredicateInput(*body.Predicate)
	}
	if body.Priority != nil {
		input.Priority = *body.Priority
	}
	if body.IsActive != nil {
		input.IsActive = *body.IsActive
	}
	return input, nil
}
func merchandisingActionInput(body apicontract.SearchMerchandisingActionInput) (searchservice.MerchandisingAction, error) {
	action := searchservice.MerchandisingAction{Multiplier: body.Multiplier, Targets: make([]searchservice.MerchandisingTarget, 0, len(body.Targets))}
	for _, target := range body.Targets {
		if target.ProductId == nil {
			return action, problemError(http.StatusBadRequest, "invalid_request", "Every merchandising target requires product_id.", nil)
		}
		if *target.ProductId < 1 {
			return action, searchConfigurationEndpointError(fmt.Errorf("%w: target product IDs must be positive", searchservice.ErrConfigurationInvalid))
		}
		action.Targets = append(action.Targets, searchservice.MerchandisingTarget{ProductID: uint(*target.ProductId), Position: target.Position})
	}
	return action, nil
}
func merchandisingPredicateInput(body apicontract.SearchMerchandisingPredicate) searchservice.MerchandisingPredicate {
	input := searchservice.MerchandisingPredicate{}
	if body.Query != nil {
		input.Query = &searchservice.MerchandisingQuery{Mode: string(body.Query.Mode), Value: body.Query.Value}
	}
	if body.CategorySlugs != nil {
		input.CategorySlugs = append([]string(nil), (*body.CategorySlugs)...)
	}
	if body.Channel != nil {
		input.Channel = string(*body.Channel)
	}
	return input
}
func merchandisingRuleContract(value searchservice.MerchandisingRule) apicontract.SearchMerchandisingRule {
	predicate := apicontract.SearchMerchandisingPredicate{}
	if value.Predicate.Query != nil {
		predicate.Query = &apicontract.SearchMerchandisingQuery{Mode: apicontract.SearchMerchandisingQueryMode(value.Predicate.Query.Mode), Value: value.Predicate.Query.Value}
	}
	if len(value.Predicate.CategorySlugs) > 0 {
		categories := value.Predicate.CategorySlugs
		predicate.CategorySlugs = &categories
	}
	if value.Predicate.Channel != "" {
		channel := apicontract.SearchMerchandisingPredicateChannel(value.Predicate.Channel)
		predicate.Channel = &channel
	}
	targets := make([]apicontract.SearchMerchandisingTarget, 0, len(value.Action.Targets))
	for _, target := range value.Action.Targets {
		targets = append(targets, apicontract.SearchMerchandisingTarget{ProductId: int(target.ProductID), ProductName: target.ProductName, Position: target.Position})
	}
	return apicontract.SearchMerchandisingRule{Id: int(value.ID), Name: value.Name, RuleType: apicontract.SearchMerchandisingRuleRuleType(value.RuleType), Predicate: predicate, Action: apicontract.SearchMerchandisingAction{Targets: targets, Multiplier: value.Action.Multiplier}, Priority: value.Priority, StartsAt: value.StartsAt, EndsAt: value.EndsAt, IsActive: value.IsActive, Version: value.Version, UpdatedBy: searchConfigurationUpdatedBy(value.UpdatedBy), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func previewSearchParams(body apicontract.SearchPreviewFilters) apicontract.SearchProductsParams {
	params := apicontract.SearchProductsParams{Q: body.Q, MinPrice: body.MinPrice, MaxPrice: body.MaxPrice, BrandSlug: body.BrandSlug, CategorySlug: body.CategorySlug, HasVariantStock: body.HasVariantStock, PriceRange: body.PriceRange, Attribute: body.Attribute, RankingProfile: body.RankingProfile, Page: body.Page, Limit: body.Limit}
	if body.Sort != nil {
		sort := apicontract.SearchProductsParamsSort(*body.Sort)
		params.Sort = &sort
	}
	if body.Order != nil {
		order := apicontract.SearchProductsParamsOrder(*body.Order)
		params.Order = &order
	}
	return params
}
