package httpapi

import (
	"context"
	"errors"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/services/accountdata"
)

func isUnauthorizedProblem(err error) bool {
	var problem *ProblemError
	return errors.As(err, &problem) && problem.Problem.Status == http.StatusUnauthorized
}

func (s *AccountEndpoints) ListSavedAddresses(ctx context.Context, _ apicontract.ListSavedAddressesRequestObject) (apicontract.ListSavedAddressesResponseObject, error) {
	user, err := s.principalUser(ctx)
	if err != nil {
		if isUnauthorizedProblem(err) {
			problem := s.contractProblem(ctx, http.StatusUnauthorized, err)
			return apicontract.ListSavedAddresses401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.ListSavedAddresses500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	addresses, err := s.accountData.ListSavedAddresses(ctx, user.ID)
	if err != nil {
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.ListSavedAddresses500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	response := make(apicontract.ListSavedAddresses200JSONResponse, 0, len(addresses))
	for _, address := range addresses {
		response = append(response, modelAddress(address))
	}
	return response, nil
}

func (s *AccountEndpoints) CreateSavedAddress(ctx context.Context, request apicontract.CreateSavedAddressRequestObject) (apicontract.CreateSavedAddressResponseObject, error) {
	user, err := s.principalUser(ctx)
	if err != nil {
		if isUnauthorizedProblem(err) {
			problem := s.contractProblem(ctx, http.StatusUnauthorized, err)
			return apicontract.CreateSavedAddress401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.CreateSavedAddress500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if request.Body == nil {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_request", "An address body is required.", nil))
		return apicontract.CreateSavedAddress400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	body := request.Body
	address, err := s.accountData.CreateSavedAddress(ctx, user.ID, accountdata.CreateSavedAddressInput{Label: stringValue(body.Label), FullName: body.FullName, Line1: body.Line1, Line2: stringValue(body.Line2), City: body.City, State: stringValue(body.State), PostalCode: body.PostalCode, Country: body.Country, Phone: stringValue(body.Phone), SetDefault: boolValue(body.SetDefault)})
	if err != nil {
		if errors.Is(err, accountdata.ErrInvalidAddress) || errors.Is(err, accountdata.ErrInvalidCountry) {
			problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_address", err.Error(), err))
			return apicontract.CreateSavedAddress400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.CreateSavedAddress500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.CreateSavedAddress201JSONResponse(modelAddress(address)), nil
}

func (s *AccountEndpoints) DeleteSavedAddress(ctx context.Context, request apicontract.DeleteSavedAddressRequestObject) (apicontract.DeleteSavedAddressResponseObject, error) {
	user, err := s.principalUser(ctx)
	if err != nil {
		if isUnauthorizedProblem(err) {
			problem := s.contractProblem(ctx, http.StatusUnauthorized, err)
			return apicontract.DeleteSavedAddress401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.DeleteSavedAddress500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if request.Id < 1 {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_address_id", "Address ID must be positive.", nil))
		return apicontract.DeleteSavedAddress400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err := s.accountData.DeleteSavedAddress(ctx, user.ID, uint(request.Id)); err != nil {
		if errors.Is(err, accountdata.ErrAddressNotFound) {
			problem := s.contractProblem(ctx, http.StatusNotFound, problemError(http.StatusNotFound, "address_not_found", "Saved address not found.", err))
			return apicontract.DeleteSavedAddress404ApplicationProblemPlusJSONResponse{NotFoundProblemApplicationProblemPlusJSONResponse: apicontract.NotFoundProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.DeleteSavedAddress500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.DeleteSavedAddress200JSONResponse{Message: "Address deleted"}, nil
}

func (s *AccountEndpoints) SetDefaultAddress(ctx context.Context, request apicontract.SetDefaultAddressRequestObject) (apicontract.SetDefaultAddressResponseObject, error) {
	user, err := s.principalUser(ctx)
	if err != nil {
		if isUnauthorizedProblem(err) {
			problem := s.contractProblem(ctx, http.StatusUnauthorized, err)
			return apicontract.SetDefaultAddress401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.SetDefaultAddress500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if request.Id < 1 {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_address_id", "Address ID must be positive.", nil))
		return apicontract.SetDefaultAddress400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	address, err := s.accountData.SetDefaultAddress(ctx, user.ID, uint(request.Id))
	if err != nil {
		if errors.Is(err, accountdata.ErrAddressNotFound) {
			problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "address_not_found", "Saved address not found.", err))
			return apicontract.SetDefaultAddress400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.SetDefaultAddress500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.SetDefaultAddress200JSONResponse(modelAddress(address)), nil
}

func (s *AccountEndpoints) ListSavedPaymentMethods(ctx context.Context, _ apicontract.ListSavedPaymentMethodsRequestObject) (apicontract.ListSavedPaymentMethodsResponseObject, error) {
	user, err := s.principalUser(ctx)
	if err != nil {
		if isUnauthorizedProblem(err) {
			problem := s.contractProblem(ctx, http.StatusUnauthorized, err)
			return apicontract.ListSavedPaymentMethods401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.ListSavedPaymentMethods500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	methods, err := s.accountData.ListSavedPaymentMethods(ctx, user.ID)
	if err != nil {
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.ListSavedPaymentMethods500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	response := make(apicontract.ListSavedPaymentMethods200JSONResponse, 0, len(methods))
	for _, method := range methods {
		response = append(response, modelPaymentMethod(method))
	}
	return response, nil
}

func (s *AccountEndpoints) CreateSavedPaymentMethod(ctx context.Context, request apicontract.CreateSavedPaymentMethodRequestObject) (apicontract.CreateSavedPaymentMethodResponseObject, error) {
	user, err := s.principalUser(ctx)
	if err != nil {
		if isUnauthorizedProblem(err) {
			problem := s.contractProblem(ctx, http.StatusUnauthorized, err)
			return apicontract.CreateSavedPaymentMethod401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.CreateSavedPaymentMethod500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if request.Body == nil {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_request", "A payment method body is required.", nil))
		return apicontract.CreateSavedPaymentMethod400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	body := request.Body
	method, err := s.accountData.CreateSavedPaymentMethod(ctx, user.ID, accountdata.CreateSavedPaymentMethodInput{CardholderName: body.CardholderName, CardNumber: body.CardNumber, ExpMonth: body.ExpMonth, ExpYear: body.ExpYear, Nickname: stringValue(body.Nickname), SetDefault: boolValue(body.SetDefault)})
	if err != nil {
		if errors.Is(err, accountdata.ErrInvalidCardholderName) || errors.Is(err, accountdata.ErrInvalidCardNumber) || errors.Is(err, accountdata.ErrInvalidExpirationMonth) || errors.Is(err, accountdata.ErrInvalidExpirationYear) {
			problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_payment_method", err.Error(), err))
			return apicontract.CreateSavedPaymentMethod400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.CreateSavedPaymentMethod500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.CreateSavedPaymentMethod201JSONResponse(modelPaymentMethod(method)), nil
}

func (s *AccountEndpoints) DeleteSavedPaymentMethod(ctx context.Context, request apicontract.DeleteSavedPaymentMethodRequestObject) (apicontract.DeleteSavedPaymentMethodResponseObject, error) {
	user, err := s.principalUser(ctx)
	if err != nil {
		if isUnauthorizedProblem(err) {
			problem := s.contractProblem(ctx, http.StatusUnauthorized, err)
			return apicontract.DeleteSavedPaymentMethod401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.DeleteSavedPaymentMethod500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if request.Id < 1 {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_payment_method_id", "Payment method ID must be positive.", nil))
		return apicontract.DeleteSavedPaymentMethod400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err := s.accountData.DeleteSavedPaymentMethod(ctx, user.ID, uint(request.Id)); err != nil {
		if errors.Is(err, accountdata.ErrPaymentMethodNotFound) {
			problem := s.contractProblem(ctx, http.StatusNotFound, problemError(http.StatusNotFound, "payment_method_not_found", "Saved payment method not found.", err))
			return apicontract.DeleteSavedPaymentMethod404ApplicationProblemPlusJSONResponse{NotFoundProblemApplicationProblemPlusJSONResponse: apicontract.NotFoundProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.DeleteSavedPaymentMethod500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.DeleteSavedPaymentMethod200JSONResponse{Message: "Payment method deleted"}, nil
}

func (s *AccountEndpoints) SetDefaultPaymentMethod(ctx context.Context, request apicontract.SetDefaultPaymentMethodRequestObject) (apicontract.SetDefaultPaymentMethodResponseObject, error) {
	user, err := s.principalUser(ctx)
	if err != nil {
		if isUnauthorizedProblem(err) {
			problem := s.contractProblem(ctx, http.StatusUnauthorized, err)
			return apicontract.SetDefaultPaymentMethod401ApplicationProblemPlusJSONResponse{AuthenticationRequiredProblemApplicationProblemPlusJSONResponse: apicontract.AuthenticationRequiredProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.SetDefaultPaymentMethod500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if request.Id < 1 {
		problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "invalid_payment_method_id", "Payment method ID must be positive.", nil))
		return apicontract.SetDefaultPaymentMethod400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	method, err := s.accountData.SetDefaultPaymentMethod(ctx, user.ID, uint(request.Id))
	if err != nil {
		if errors.Is(err, accountdata.ErrPaymentMethodNotFound) {
			problem := s.contractProblem(ctx, http.StatusBadRequest, problemError(http.StatusBadRequest, "payment_method_not_found", "Saved payment method not found.", err))
			return apicontract.SetDefaultPaymentMethod400ApplicationProblemPlusJSONResponse{BadRequestProblemApplicationProblemPlusJSONResponse: apicontract.BadRequestProblemApplicationProblemPlusJSONResponse(problem)}, nil
		}
		problem := s.contractProblem(ctx, http.StatusInternalServerError, err)
		return apicontract.SetDefaultPaymentMethod500ApplicationProblemPlusJSONResponse{InternalServerErrorProblemApplicationProblemPlusJSONResponse: apicontract.InternalServerErrorProblemApplicationProblemPlusJSONResponse(problem)}, nil
	}
	return apicontract.SetDefaultPaymentMethod200JSONResponse(modelPaymentMethod(method)), nil
}
