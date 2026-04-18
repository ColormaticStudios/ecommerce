package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"ecommerce/handlers"
	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	catalogadmin "ecommerce/internal/services/catalogadmin"

	"github.com/pmezard/go-difflib/difflib"
	"gorm.io/gorm"
)

const productSEOEntityType = "product"

func loadCurrentProductUpsertInput(db *gorm.DB, mediaService *media.Service, productID uint) (apicontract.ProductUpsertInput, error) {
	contract, err := invokeLocalJSON[apicontract.Product](handlers.GetAdminProductByID(db, mediaService), localHandlerRequest{
		Method:     "GET",
		Path:       fmt.Sprintf("/api/v1/admin/products/%d", productID),
		PathParams: map[string]string{"id": fmt.Sprintf("%d", productID)},
	})
	if err != nil {
		return apicontract.ProductUpsertInput{}, err
	}
	return catalogadmin.ProductContractToUpsertInput(contract), nil
}

func loadLiveProductUpsertInput(db *gorm.DB, mediaService *media.Service, productID uint) (apicontract.ProductUpsertInput, error) {
	return catalogadmin.LoadLiveProductUpsertInput(db, mediaService, productID)
}

func productContractToUpsertInput(product apicontract.Product) apicontract.ProductUpsertInput {
	return catalogadmin.ProductContractToUpsertInput(product)
}

func buildUnifiedJSONDiff(from any, to any, fromName string, toName string) (string, error) {
	fromJSON, err := json.MarshalIndent(from, "", "  ")
	if err != nil {
		return "", err
	}
	toJSON, err := json.MarshalIndent(to, "", "  ")
	if err != nil {
		return "", err
	}

	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(fromJSON) + "\n"),
		B:        difflib.SplitLines(string(toJSON) + "\n"),
		FromFile: fromName,
		ToFile:   toName,
		Context:  3,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimRight(diff, "\n"), nil
}
