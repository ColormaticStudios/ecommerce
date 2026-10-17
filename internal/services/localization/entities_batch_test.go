package localization

import (
	"context"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResolveEntitiesMatchesSingleEntityFallbackAndSupportsAttributeHeadings(t *testing.T) {
	s, db := newLocalizationTestService(t)
	ctx := context.Background()
	require.NoError(t, db.AutoMigrate(&models.ProductAttribute{}))
	_, err := s.ReplaceLocales(ctx, []LocaleInput{{Code: "en-US", Name: "English", IsDefault: true, IsEnabled: true}, {Code: "fr", Name: "French", IsEnabled: true}})
	require.NoError(t, err)
	brand := models.Brand{Name: "Canonical name", Slug: "brand"}
	attribute := models.ProductAttribute{Key: "Color", Slug: "color", Type: "enum", Filterable: true}
	require.NoError(t, db.Create(&brand).Error)
	require.NoError(t, db.Create(&attribute).Error)
	_, err = s.PutEntityLocalization(ctx, EntityTypeBrand, brand.ID, "en-US", map[string]string{"name": "Brand", "description": "English description"}, nil)
	require.NoError(t, err)
	_, err = s.PutEntityLocalization(ctx, EntityTypeBrand, brand.ID, "fr", map[string]string{"name": "Marque"}, nil)
	require.NoError(t, err)
	_, err = s.PutEntityLocalization(ctx, EntityTypeProductAttribute, attribute.ID, "en-US", map[string]string{"key": "Color"}, nil)
	require.NoError(t, err)
	_, err = s.PutEntityLocalization(ctx, EntityTypeProductAttribute, attribute.ID, "fr", map[string]string{"key": "Couleur"}, nil)
	require.NoError(t, err)
	refs := []EntityReference{{EntityTypeBrand, brand.ID}, {EntityTypeProductAttribute, attribute.ID}}
	values, err := s.ResolveEntities(ctx, refs, ResolutionInput{ExplicitLocale: "fr"})
	require.NoError(t, err)
	single, err := s.EntityLocalizations(ctx, EntityTypeBrand, brand.ID, ResolutionInput{ExplicitLocale: "fr"})
	require.NoError(t, err)
	require.Equal(t, single.Resolved, values[refs[0]])
	require.Equal(t, "Couleur", values[refs[1]].Fields["key"])
	_, err = s.PutEntityLocalization(ctx, EntityTypeProductAttribute, attribute.ID, "fr", map[string]string{"name": "unsupported"}, nil)
	require.ErrorIs(t, err, ErrInvalidTranslation)
	for _, bad := range []EntityReference{{EntityTypeBrand, 9999}, {"unknown", 1}, {EntityTypeBrand, 0}} {
		_, err = s.ResolveEntities(ctx, []EntityReference{bad}, ResolutionInput{})
		require.Error(t, err)
	}
}
