package localization

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"ecommerce/internal/media"
	"ecommerce/internal/requestctx"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var localizationTestDBSequence atomic.Uint64

func newLocalizationTestService(t testing.TB) (*Service, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), localizationTestDBSequence.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Locale{},
		&models.LocaleMarketDefault{},
		&models.TranslationKey{},
		&models.TranslationKeyUsage{},
		&models.TranslationValue{},
		&models.TranslationRelease{},
		&models.TranslationReleaseEntry{},
		&models.LocalizedEntityValue{},
		&models.TranslationComment{},
		&models.TranslationAuditEvent{},
		&models.LocalizationGlossaryTerm{},
		&models.LocalizationRoleAssignment{},
		&models.LocalizationRollout{},
		&models.LocalizationMetric{},
		&models.Brand{},
		&models.User{},
		&models.MediaObject{},
		&models.MediaReference{},
	))
	return NewService(db), db
}

func TestTranslationKeyUsagesAttachMediaManagedScreenshots(t *testing.T) {
	service, db := newLocalizationTestService(t)
	mediaService := media.NewService(db, t.TempDir(), "/media", nil, nil)
	key, err := service.CreateKey(context.Background(), KeyInput{Namespace: "storefront", Key: "usage.test", SourceText: "Usage", OwnerDomain: "storefront"})
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.MediaObject{ID: "shot", OriginalPath: "shot.webp", MimeType: "image/webp", Status: media.StatusReady}).Error)

	values, err := service.ReplaceUsages(context.Background(), key.ID, []UsageInput{{Route: "/cart", Component: "Cart.svelte", Description: "Empty state", ScreenshotMediaID: "shot"}}, mediaService)
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.Equal(t, "shot", values[0].ScreenshotMediaID)
	require.Equal(t, "/media/shot.webp", values[0].ScreenshotURL)

	var refs []models.MediaReference
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ?", media.OwnerTypeTranslationUsage, values[0].ID).Find(&refs).Error)
	require.Len(t, refs, 1)
	require.Equal(t, media.RoleTranslationScreenshot, refs[0].Role)
}

func adminLocalizationContext(subject string) context.Context {
	return requestctx.WithPrincipal(context.Background(), requestctx.Principal{Subject: subject, AccountID: 7, Roles: []string{"admin"}})
}

func configureTestLocales(t testing.TB, service *Service) {
	t.Helper()
	_, err := service.ReplaceLocales(context.Background(), []LocaleInput{
		{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true},
		{Code: "fr", Name: "French", IsEnabled: true, FallbackLocale: "en-US"},
		{Code: "fr-CA", Name: "French (Canada)", IsEnabled: true, FallbackLocale: "fr"},
	})
	require.NoError(t, err)
}

func TestReplaceLocalesValidatesDefaultFallbacksAndCanonicalCodes(t *testing.T) {
	service, _ := newLocalizationTestService(t)

	_, err := service.ReplaceLocales(context.Background(), []LocaleInput{
		{Code: "en-us", Name: "English", IsEnabled: true, IsDefault: true, FallbackLocale: "fr-fr"},
		{Code: "fr-fr", Name: "French", IsEnabled: true, FallbackLocale: "en-us"},
	})
	require.ErrorIs(t, err, ErrInvalidLocale)

	_, err = service.ReplaceLocales(context.Background(), []LocaleInput{
		{Code: "en-us", Name: "English", IsEnabled: true, IsDefault: true},
		{Code: "fr-ca", Name: "French (Canada)", IsEnabled: true, FallbackLocale: "en-us"},
	})
	require.NoError(t, err)
	locales, err := service.ListLocales(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, "en-US", locales[0].Code)
	require.Equal(t, "fr-CA", locales[1].Code)
	require.Equal(t, "en-US", locales[1].FallbackLocale)

	_, err = service.ReplaceLocales(context.Background(), []LocaleInput{
		{Code: "en-US", Name: "English", IsEnabled: false, IsDefault: true},
	})
	require.ErrorIs(t, err, ErrInvalidLocale)

	_, err = service.ReplaceLocales(context.Background(), []LocaleInput{
		{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true},
		{Code: "fr", Name: "French", IsEnabled: false, DefaultMarkets: []string{"CA"}},
	})
	require.ErrorIs(t, err, ErrInvalidLocale)
}

func TestResolveLocaleUsesDocumentedPrecedenceForAnonymousAndAccounts(t *testing.T) {
	service, _ := newLocalizationTestService(t)
	configureTestLocales(t, service)

	anonymous, err := service.ResolveLocale(context.Background(), ResolutionInput{MarketDefault: "fr"})
	require.NoError(t, err)
	require.Equal(t, "fr", anonymous.ResolvedLocale)
	require.Equal(t, "market", anonymous.Source)
	require.Equal(t, []string{"fr", "en-US"}, anonymous.FallbackChain)

	authenticated, err := service.ResolveLocale(context.Background(), ResolutionInput{
		ExplicitLocale: "de-DE", AccountPreference: "fr-CA", MarketDefault: "fr",
	})
	require.NoError(t, err)
	require.Equal(t, "de-DE", authenticated.RequestedLocale)
	require.Equal(t, "fr-CA", authenticated.ResolvedLocale)
	require.Equal(t, "account", authenticated.Source)
	require.True(t, authenticated.UsedFallback)
	require.Equal(t, []string{"fr-CA", "fr", "en-US"}, authenticated.FallbackChain)

	global, err := service.ResolveLocale(context.Background(), ResolutionInput{})
	require.NoError(t, err)
	require.Equal(t, "en-US", global.ResolvedLocale)
	require.Equal(t, "global", global.Source)
}

func TestTranslationWorkflowReleaseActivationAndFallbackLookup(t *testing.T) {
	service, _ := newLocalizationTestService(t)
	configureTestLocales(t, service)
	key, err := service.CreateKey(context.Background(), KeyInput{
		Namespace: "checkout", Key: "cart.empty", SourceText: "Your cart is empty.", OwnerDomain: "checkout",
	})
	require.NoError(t, err)

	english, err := service.CreateValue(context.Background(), key.ID, "en-US", "Your cart is empty.", nil)
	require.NoError(t, err)
	_, err = service.TransitionValue(context.Background(), english.ID, models.TranslationStatePublished, nil)
	require.ErrorIs(t, err, ErrInvalidTransition)
	english, err = service.TransitionValue(context.Background(), english.ID, models.TranslationStateReview, nil)
	require.NoError(t, err)
	_, err = service.TransitionValue(context.Background(), english.ID, models.TranslationStatePublished, nil)
	require.NoError(t, err)

	french, err := service.CreateValue(context.Background(), key.ID, "fr", "Votre panier est vide.", nil)
	require.NoError(t, err)
	french, err = service.TransitionValue(context.Background(), french.ID, models.TranslationStateReview, nil)
	require.NoError(t, err)
	_, err = service.TransitionValue(context.Background(), french.ID, models.TranslationStatePublished, nil)
	require.NoError(t, err)

	releaseOne, err := service.CreateRelease(context.Background(), "release-1", "initial")
	require.NoError(t, err)
	_, err = service.ActivateRelease(context.Background(), releaseOne.ID, nil)
	require.NoError(t, err)
	bundleOne, err := service.Bundle(context.Background(), ResolutionInput{ExplicitLocale: "fr"}, "checkout")
	require.NoError(t, err)
	require.Len(t, bundleOne.Release.Version, 64)
	lookup, err := service.Lookup(context.Background(), "checkout", "cart.empty", ResolutionInput{ExplicitLocale: "fr-CA"})
	require.NoError(t, err)
	require.Equal(t, "Votre panier est vide.", lookup.Value)
	require.Equal(t, "fr", lookup.SourceLocale)
	require.True(t, lookup.UsedFallback)
	require.False(t, lookup.MissingTranslation)

	second, err := service.CreateValue(context.Background(), key.ID, "fr", "Le panier est vide.", nil)
	require.NoError(t, err)
	second, err = service.TransitionValue(context.Background(), second.ID, models.TranslationStateReview, nil)
	require.NoError(t, err)
	_, err = service.TransitionValue(context.Background(), second.ID, models.TranslationStatePublished, nil)
	require.NoError(t, err)
	lookup, err = service.Lookup(context.Background(), "checkout", "cart.empty", ResolutionInput{ExplicitLocale: "fr"})
	require.NoError(t, err)
	require.Equal(t, "Votre panier est vide.", lookup.Value, "active release must remain immutable")

	releaseTwo, err := service.CreateRelease(context.Background(), "release-2", "updated French")
	require.NoError(t, err)
	_, err = service.ActivateRelease(context.Background(), releaseTwo.ID, nil)
	require.NoError(t, err)
	bundleTwo, err := service.Bundle(context.Background(), ResolutionInput{ExplicitLocale: "fr"}, "checkout")
	require.NoError(t, err)
	require.NotEqual(t, bundleOne.Release.Version, bundleTwo.Release.Version)
	lookup, err = service.Lookup(context.Background(), "checkout", "cart.empty", ResolutionInput{ExplicitLocale: "fr"})
	require.NoError(t, err)
	require.Equal(t, "Le panier est vide.", lookup.Value)
}

func TestReleaseActivationRecordsRestoredSnapshotAsRollback(t *testing.T) {
	service, db := newLocalizationTestService(t)
	configureTestLocales(t, service)
	ctx := adminLocalizationContext("publisher@example.com")
	key, err := service.CreateKey(ctx, KeyInput{
		Namespace: "storefront", Key: "rollback.test", SourceText: "Original", OwnerDomain: "storefront",
	})
	require.NoError(t, err)
	value, err := service.CreateValue(ctx, key.ID, "en-US", "Original", nil)
	require.NoError(t, err)
	value, err = service.TransitionValue(ctx, value.ID, models.TranslationStateReview, nil)
	require.NoError(t, err)
	_, err = service.TransitionValue(ctx, value.ID, models.TranslationStatePublished, nil)
	require.NoError(t, err)

	original, err := service.CreateReleaseWithAudit(ctx, "original", "", nil)
	require.NoError(t, err)
	_, err = service.ActivateReleaseWithAudit(ctx, original.ID, nil)
	require.NoError(t, err)

	updated, err := service.CreateValue(ctx, key.ID, "en-US", "Updated", nil)
	require.NoError(t, err)
	updated, err = service.TransitionValue(ctx, updated.ID, models.TranslationStateReview, nil)
	require.NoError(t, err)
	_, err = service.TransitionValue(ctx, updated.ID, models.TranslationStatePublished, nil)
	require.NoError(t, err)
	updatedRelease, err := service.CreateReleaseWithAudit(ctx, "updated", "", nil)
	require.NoError(t, err)
	_, err = service.ActivateReleaseWithAudit(ctx, updatedRelease.ID, nil)
	require.NoError(t, err)
	restored, err := service.RollbackReleaseWithAudit(ctx, original.ID, nil)
	require.NoError(t, err)
	require.NotEqual(t, original.ID, restored.ID)
	require.Equal(t, original.SnapshotHash, restored.SnapshotHash)
	require.Equal(t, models.TranslationReleaseStatusActive, restored.Status)

	metrics, err := service.Metrics(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, metrics.RollbackCount)
	var event models.TranslationAuditEvent
	require.NoError(t, db.Where("release_id = ?", restored.ID).Order("id DESC").First(&event).Error)
	require.Equal(t, "release_rollback_activated", event.Action)
}

func TestReleaseQualityBlocksCriticalSourceTextFallback(t *testing.T) {
	service, _ := newLocalizationTestService(t)
	configureTestLocales(t, service)
	_, err := service.CreateKey(context.Background(), KeyInput{
		Namespace: "errors", Key: "payment.declined", SourceText: "Payment was declined.", OwnerDomain: "errors",
	})
	require.NoError(t, err)
	release, err := service.CreateRelease(context.Background(), "incomplete", "")
	require.NoError(t, err)
	quality, err := service.ReleaseQuality(context.Background(), release.ID)
	require.NoError(t, err)
	require.False(t, quality.Ready)
	require.Len(t, quality.Missing, 3)
	_, err = service.ActivateRelease(context.Background(), release.ID, nil)
	require.ErrorIs(t, err, ErrReleaseQualityFailed)
}

func TestBundleReturnsExplicitSourceFallbackInsteadOfEmptyText(t *testing.T) {
	service, _ := newLocalizationTestService(t)
	configureTestLocales(t, service)
	_, err := service.CreateKey(context.Background(), KeyInput{
		Namespace: "storefront", Key: "payment.declined", SourceText: "Payment was declined.", OwnerDomain: "storefront",
	})
	require.NoError(t, err)
	release, err := service.CreateRelease(context.Background(), "source-only", "")
	require.NoError(t, err)
	_, err = service.ActivateRelease(context.Background(), release.ID, nil)
	require.NoError(t, err)

	result, err := service.Lookup(context.Background(), "storefront", "payment.declined", ResolutionInput{ExplicitLocale: "fr-CA"})
	require.NoError(t, err)
	require.Equal(t, "Payment was declined.", result.Value)
	require.Equal(t, "en-US", result.SourceLocale)
	require.True(t, result.UsedFallback)
	require.True(t, result.MissingTranslation)

	_, err = service.Lookup(context.Background(), "storefront", "unknown", ResolutionInput{ExplicitLocale: "en-US"})
	require.ErrorIs(t, err, ErrTranslationKeyNotFound)
}

func TestEntityLocalizationResolvesEachFieldAcrossFallbackChain(t *testing.T) {
	service, db := newLocalizationTestService(t)
	configureTestLocales(t, service)
	description := "Handmade carryall"
	brand := models.Brand{Name: "Northstar", Slug: "northstar", Description: &description, IsActive: true}
	require.NoError(t, db.Select("*").Create(&brand).Error)

	_, err := service.PutEntityLocalization(context.Background(), EntityTypeBrand, brand.ID, "en-US", map[string]string{
		"name": "Northstar", "description": description,
	}, nil)
	require.NoError(t, err)
	_, err = service.PutEntityLocalization(context.Background(), EntityTypeBrand, brand.ID, "fr", map[string]string{
		"name": "Étoile du Nord",
	}, nil)
	require.NoError(t, err)

	result, err := service.EntityLocalizations(context.Background(), EntityTypeBrand, brand.ID, ResolutionInput{ExplicitLocale: "fr-CA"})
	require.NoError(t, err)
	require.Equal(t, "Étoile du Nord", result.Resolved.Fields["name"])
	require.Equal(t, description, result.Resolved.Fields["description"])
	require.Equal(t, "fr", result.Resolved.SourceLocales["name"])
	require.Equal(t, "en-US", result.Resolved.SourceLocales["description"])
	require.True(t, result.Resolved.UsedFallback)

	_, err = service.PutEntityLocalization(context.Background(), EntityTypeBrand, brand.ID, "fr", map[string]string{
		"name": "Nord",
	}, nil)
	require.NoError(t, err)
	result, err = service.EntityLocalizations(context.Background(), EntityTypeBrand, brand.ID, ResolutionInput{ExplicitLocale: "fr"})
	require.NoError(t, err)
	require.Equal(t, "Nord", result.Resolved.Fields["name"])

	_, err = service.PutEntityLocalization(context.Background(), EntityTypeBrand, brand.ID, "fr", map[string]string{"unknown": "no"}, nil)
	require.ErrorIs(t, err, ErrInvalidTranslation)
}

func TestWorkflowValidationBlocksReviewAndRecordsAuditedTransitions(t *testing.T) {
	service, db := newLocalizationTestService(t)
	configureTestLocales(t, service)
	ctx := adminLocalizationContext("publisher@example.com")
	key, err := service.CreateKey(ctx, KeyInput{Namespace: "checkout", Key: "welcome", SourceText: "Welcome, {customer}", OwnerDomain: "checkout"})
	require.NoError(t, err)

	invalid, err := service.CreateValueWithOptions(ctx, key.ID, "fr", "Bienvenue", uintPointer(7), ValueOptions{ExpectedVersion: uintPointer(0), ChangeSummary: "Initial translation"})
	require.NoError(t, err)
	issues, err := service.ValidateValue(ctx, invalid.ID)
	require.NoError(t, err)
	require.Equal(t, "placeholder_mismatch", issues[0].Code)
	_, err = service.TransitionValueWithAudit(ctx, invalid.ID, models.TranslationStateReview, uintPointer(7), "Ready")
	require.ErrorIs(t, err, ErrInvalidTranslation)

	valid, err := service.CreateValueWithOptions(ctx, key.ID, "fr", "Bienvenue, {customer}", uintPointer(7), ValueOptions{ExpectedVersion: uintPointer(1), ChangeSummary: "Preserve placeholder"})
	require.NoError(t, err)
	valid, err = service.TransitionValueWithAudit(ctx, valid.ID, models.TranslationStateReview, uintPointer(7), "Editor approved")
	require.NoError(t, err)
	_, err = service.TransitionValueWithAudit(ctx, valid.ID, models.TranslationStatePublished, uintPointer(7), "Publisher approved")
	require.NoError(t, err)

	var audit []models.TranslationAuditEvent
	require.NoError(t, db.Where("translation_value_id = ?", valid.ID).Order("created_at ASC").Find(&audit).Error)
	require.Len(t, audit, 3)
	require.Equal(t, []string{"draft_created", "value_review", "value_published"}, []string{audit[0].Action, audit[1].Action, audit[2].Action})
	require.Equal(t, "publisher@example.com", audit[2].ActorSubject)
}

func TestWorkflowRolesGlossaryQueueAndImportConflict(t *testing.T) {
	service, _ := newLocalizationTestService(t)
	configureTestLocales(t, service)
	publisher := adminLocalizationContext("publisher@example.com")
	translator := adminLocalizationContext("translator@example.com")
	_, err := service.PutRole(publisher, "translator@example.com", models.LocalizationRoleTranslator)
	require.NoError(t, err)
	require.NoError(t, service.RequireRole(translator, models.LocalizationRoleTranslator))
	require.ErrorIs(t, service.RequireRole(translator, models.LocalizationRoleEditor), ErrLocalizationForbidden)
	require.NoError(t, service.RequireRole(publisher, models.LocalizationRolePublisher))

	_, err = service.PutGlossary(publisher, "fr", "Checkout", "Passer la commande", "Approved term", true)
	require.NoError(t, err)
	key, err := service.CreateKey(publisher, KeyInput{Namespace: "storefront", Key: "cart.checkout", SourceText: "Checkout", OwnerDomain: "storefront"})
	require.NoError(t, err)
	draft, err := service.CreateValue(publisher, key.ID, "fr", "Paiement", uintPointer(7))
	require.NoError(t, err)
	issues, err := service.ValidateValue(publisher, draft.ID)
	require.NoError(t, err)
	require.Equal(t, "glossary_term_locked", issues[0].Code)

	missingKey, err := service.CreateKey(publisher, KeyInput{Namespace: "storefront", Key: "cart.empty", SourceText: "Your cart is empty", OwnerDomain: "storefront"})
	require.NoError(t, err)
	queue, err := service.ListQueue(translator, QueueFilter{Locale: "fr", Missing: boolPointer(true)})
	require.NoError(t, err)
	require.Len(t, queue.Items, 1)
	require.Equal(t, missingKey.ID, queue.Items[0].Key.ID)

	dryRun, err := service.Import(translator, "fr", "storefront", "json", `{"storefront.cart.empty":"Votre panier est vide"}`, true, uintPointer(7))
	require.NoError(t, err)
	require.Equal(t, 1, dryRun.Created)
	created, err := service.Import(translator, "fr", "storefront", "json", `{"storefront.cart.empty":"Votre panier est vide"}`, false, uintPointer(7))
	require.NoError(t, err)
	require.Equal(t, 1, created.Created)
	conflict, err := service.Import(translator, "fr", "storefront", "json", `{"storefront.cart.empty":"Panier vide"}`, true, uintPointer(7))
	require.NoError(t, err)
	require.Equal(t, 1, conflict.Conflicts)

	exported, err := service.Export(translator, "fr", "storefront", "xliff")
	require.NoError(t, err)
	require.Contains(t, exported.Content, `version="2.0"`)
	require.Contains(t, exported.Content, "storefront.cart.empty")
}

func TestTranslationQueuePaginationAndSearch(t *testing.T) {
	service, _ := newLocalizationTestService(t)
	configureTestLocales(t, service)
	ctx := adminLocalizationContext("publisher@example.com")
	for _, input := range []KeyInput{
		{Namespace: "storefront", Key: "catalog.alpha", SourceText: "Alpha product", OwnerDomain: "catalog"},
		{Namespace: "storefront", Key: "catalog.beta", SourceText: "Beta product", OwnerDomain: "catalog"},
		{Namespace: "storefront", Key: "catalog.gamma", SourceText: "Gamma product", OwnerDomain: "catalog"},
	} {
		_, err := service.CreateKey(ctx, input)
		require.NoError(t, err)
	}

	first, err := service.ListQueue(ctx, QueueFilter{Locale: "fr", Page: 1, Limit: 2})
	require.NoError(t, err)
	require.Equal(t, 3, first.Total)
	require.Equal(t, 2, first.TotalPages)
	require.Equal(t, 1, first.Page)
	require.Equal(t, 2, first.Limit)
	require.Len(t, first.Items, 2)
	require.Equal(t, "catalog.alpha", first.Items[0].Key.Key)
	require.Equal(t, "catalog.beta", first.Items[1].Key.Key)

	second, err := service.ListQueue(ctx, QueueFilter{Locale: "fr", Page: 2, Limit: 2})
	require.NoError(t, err)
	require.Equal(t, 3, second.Total)
	require.Len(t, second.Items, 1)
	require.Equal(t, "catalog.gamma", second.Items[0].Key.Key)

	search, err := service.ListQueue(ctx, QueueFilter{Locale: "fr", Query: "storefront.catalog.beta", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, search.Total)
	require.Equal(t, 1, search.TotalPages)
	require.Len(t, search.Items, 1)
	require.Equal(t, "catalog.beta", search.Items[0].Key.Key)

	_, err = service.ListQueue(ctx, QueueFilter{Locale: "fr", Page: 1, Limit: 101})
	require.ErrorIs(t, err, ErrInvalidTranslation)
}

func TestLocalizationRolloutsCommunicationRenderingAndMetrics(t *testing.T) {
	service, _ := newLocalizationTestService(t)
	configureTestLocales(t, service)
	ctx := adminLocalizationContext("publisher@example.com")
	templates := []struct {
		key    string
		source string
		french string
	}{
		{key: "order_placed.email.subject", source: "Order {order_number} confirmed", french: "Commande {order_number} confirmée"},
		{key: "order_placed.email.body", source: "Order {order_number} total: {total}", french: "Commande {order_number}, total : {total}"},
	}
	for _, template := range templates {
		key, err := service.CreateKey(ctx, KeyInput{Namespace: "communications", Key: template.key, SourceText: template.source, OwnerDomain: "communications"})
		require.NoError(t, err)
		for _, translation := range []struct {
			locale string
			value  string
		}{{locale: "en-US", value: template.source}, {locale: "fr", value: template.french}} {
			value, err := service.CreateValue(ctx, key.ID, translation.locale, translation.value, uintPointer(7))
			require.NoError(t, err)
			value, err = service.TransitionValue(ctx, value.ID, models.TranslationStateReview, uintPointer(7))
			require.NoError(t, err)
			_, err = service.TransitionValue(ctx, value.ID, models.TranslationStatePublished, uintPointer(7))
			require.NoError(t, err)
		}
	}
	release, err := service.CreateRelease(ctx, "communications", "P4 communication templates")
	require.NoError(t, err)
	_, err = service.ActivateRelease(ctx, release.ID, uintPointer(7))
	require.NoError(t, err)

	localized, err := service.RenderCommunication(ctx, CommunicationInput{
		Event: "order_placed", Channel: "email", RecipientLocale: "fr", RecipientKey: "customer-17",
		Parameters: map[string]string{"order_number": "1007", "total": "42,00 €"},
	})
	require.NoError(t, err)
	require.Equal(t, "Commande 1007 confirmée", localized.Subject)
	require.Equal(t, "fr", localized.ResolvedLocale)
	require.False(t, localized.UsedFallback)

	rollouts, err := service.ListRollouts(ctx)
	require.NoError(t, err)
	inputs := make([]RolloutInput, 0, len(rollouts))
	for _, rollout := range rollouts {
		input := RolloutInput{Locale: rollout.Locale, Domain: rollout.Domain, IsEnabled: rollout.IsEnabled, Percentage: rollout.Percentage}
		if input.Locale == "fr" && input.Domain == "communications" {
			input.IsEnabled = false
			input.Percentage = 0
		}
		inputs = append(inputs, input)
	}
	_, err = service.ReplaceRollouts(ctx, inputs, uintPointer(7))
	require.NoError(t, err)

	fallback, err := service.RenderCommunication(ctx, CommunicationInput{
		Event: "order_placed", Channel: "email", RecipientLocale: "fr", RecipientKey: "customer-17",
		Parameters: map[string]string{"order_number": "1007", "total": "$42.00"},
	})
	require.NoError(t, err)
	require.Equal(t, "Order 1007 confirmed", fallback.Subject)
	require.Equal(t, "en-US", fallback.ResolvedLocale)
	require.Equal(t, "en-US", fallback.SourceLocale)
	require.True(t, fallback.UsedFallback)

	_, err = service.RenderCommunication(ctx, CommunicationInput{
		Event: "order_placed", Channel: "email", RecipientLocale: "en-US", RecipientKey: "customer-17",
		Parameters: map[string]string{"order_number": "1007"},
	})
	require.ErrorIs(t, err, ErrInvalidTranslation)

	metrics, err := service.Metrics(ctx)
	require.NoError(t, err)
	require.Greater(t, metrics.LookupCount, int64(0))
	require.NotEmpty(t, metrics.LocaleRates)
	require.EqualValues(t, 1, metrics.RolloutFallbackCount)
	require.EqualValues(t, 4, metrics.PublishCount)
	require.GreaterOrEqual(t, metrics.MaximumPublishLatency, int64(0))
}

func TestTranslationCommentsExposeUserDisplayName(t *testing.T) {
	service, db := newLocalizationTestService(t)
	configureTestLocales(t, service)
	user := models.User{Subject: "identity-provider|00u7f3a91", Username: "morgan", Email: "morgan@example.com", Name: "Morgan Editor", Role: "admin", Currency: "USD", Locale: "en-US"}
	require.NoError(t, db.Select("*").Create(&user).Error)
	ctx := requestctx.WithPrincipal(context.Background(), requestctx.Principal{Subject: user.Subject, AccountID: user.ID, Roles: []string{"admin"}})
	key, err := service.CreateKey(ctx, KeyInput{Namespace: "admin", Key: "comments.test", SourceText: "Comment test", OwnerDomain: "admin"})
	require.NoError(t, err)
	value, err := service.CreateValue(ctx, key.ID, "en-US", "Comment test", &user.ID)
	require.NoError(t, err)

	created, err := service.AddComment(ctx, value.ID, "Looks ready to publish.", user.Subject, &user.ID)
	require.NoError(t, err)
	require.Equal(t, "Morgan Editor", created.AuthorName)
	require.NotEqual(t, user.Subject, created.AuthorName)

	comments, err := service.ListComments(ctx, value.ID)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, "Morgan Editor", comments[0].AuthorName)
}

func TestAssigneeSearchOnlyReturnsUsersWithLocalizationAccess(t *testing.T) {
	service, db := newLocalizationTestService(t)
	configureTestLocales(t, service)
	admin := models.User{Subject: "admin-subject", Username: "morgan", Email: "morgan@example.com", Name: "Morgan Editor", Role: "admin", Currency: "USD", Locale: "en-US"}
	customer := models.User{Subject: "customer-subject", Username: "casey", Email: "casey@example.com", Name: "Casey Customer", Role: "customer", Currency: "USD", Locale: "en-US"}
	require.NoError(t, db.Select("*").Create(&admin).Error)
	require.NoError(t, db.Select("*").Create(&customer).Error)
	_, err := service.PutRole(context.Background(), admin.Subject, models.LocalizationRoleEditor)
	require.NoError(t, err)

	assignees, err := service.ListAssignees(context.Background(), "Morgan", 20)
	require.NoError(t, err)
	require.Len(t, assignees, 1)
	require.Equal(t, admin.ID, assignees[0].ID)
	require.Equal(t, "Morgan Editor", assignees[0].Name)
	require.Equal(t, models.LocalizationRoleEditor, assignees[0].LocalizationRole)

	key, err := service.CreateKey(context.Background(), KeyInput{Namespace: "admin", Key: "assignment.test", SourceText: "Assignment", OwnerDomain: "admin"})
	require.NoError(t, err)
	_, err = service.CreateValueWithOptions(context.Background(), key.ID, "en-US", "Assignment", &admin.ID, ValueOptions{AssigneeID: &customer.ID})
	require.ErrorIs(t, err, ErrInvalidTranslation)
	_, err = service.CreateValueWithOptions(context.Background(), key.ID, "en-US", "Assignment", &admin.ID, ValueOptions{AssigneeID: &admin.ID})
	require.NoError(t, err)
}

func uintPointer(value uint) *uint { return &value }
func boolPointer(value bool) *bool { return &value }
