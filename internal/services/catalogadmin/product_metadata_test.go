package catalogadmin

import (
	"context"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
)

func TestInvalidMetadataReplacementRollsBackCostAndDraft(t *testing.T) {
	for _, scenario := range []string{"unknown selection", "duplicate selection", "unknown attribute", "wrong attribute type", "duplicate option value"} {
		t.Run(scenario, func(t *testing.T) {
			service, db := unitCostTestService(t)
			require.NoError(t, db.AutoMigrate(&models.ProductAttribute{}))
			attribute := models.ProductAttribute{Key: "material", Slug: "material", Type: "text"}
			require.NoError(t, db.Create(&attribute).Error)
			cost := 12.34
			input := unitCostInput(&cost)
			input.Options = []apicontract.ProductOptionInput{{Name: "Size", Values: []apicontract.ProductOptionValueInput{{Value: "M"}}}}
			input.Variants[0].Selections = []apicontract.ProductVariantSelectionInput{{OptionName: "Size", OptionValue: "M"}}
			text := "Cotton"
			input.Attributes = []apicontract.ProductAttributeValueInput{{ProductAttributeId: int(attribute.ID), TextValue: &text}}
			product, err := service.CreateProduct(context.Background(), input)
			require.NoError(t, err)
			before, err := service.ProductInput(context.Background(), product.ID)
			require.NoError(t, err)
			updated := 25.0
			input.Variants[0].UnitCost = &updated
			switch scenario {
			case "unknown selection":
				input.Variants[0].Selections[0].OptionValue = "XL"
			case "duplicate selection":
				input.Variants[0].Selections = append(input.Variants[0].Selections, input.Variants[0].Selections[0])
			case "unknown attribute":
				input.Attributes[0].ProductAttributeId = 999
			case "wrong attribute type":
				input.Attributes[0].TextValue = nil
				value := false
				input.Attributes[0].BooleanValue = &value
			case "duplicate option value":
				input.Options[0].Values = append(input.Options[0].Values, input.Options[0].Values[0])
			}
			_, err = service.UpdateProduct(context.Background(), product.ID, input)
			require.Error(t, err)
			after, err := service.ProductInput(context.Background(), product.ID)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestProductInputRejectsMalformedDraftImagesAndOrdersMetadata(t *testing.T) {
	service, db := unitCostTestService(t)
	input := unitCostInput(nil)
	first, second := 1, 2
	input.Options = []apicontract.ProductOptionInput{{Name: "Second", Position: &second, Values: []apicontract.ProductOptionValueInput{{Value: "B", Position: &second}, {Value: "A", Position: &first}}}, {Name: "First", Position: &first, Values: []apicontract.ProductOptionValueInput{{Value: "A"}}}}
	product, err := service.CreateProduct(context.Background(), input)
	require.NoError(t, err)
	current, err := service.ProductInput(context.Background(), product.ID)
	require.NoError(t, err)
	require.Equal(t, "First", current.Options[0].Name)
	require.Equal(t, "A", current.Options[1].Values[0].Value)
	require.NoError(t, db.Model(&models.ProductDraft{}).Where("product_id = ?", product.ID).Update("images_json", "broken").Error)
	_, err = service.ProductInput(context.Background(), product.ID)
	require.ErrorContains(t, err, "decode product draft images")
}
