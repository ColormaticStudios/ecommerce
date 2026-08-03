package cms

import (
	"fmt"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func createDeliveryTestPage(t *testing.T, service *Service) (*PageRecord, *PageRecord) {
	t.Helper()
	created, err := service.CreateDraft(PageDraftInput{
		Path: "/campaign", Title: "Campaign", Payload: PagePayload{"blocks": []any{}},
	})
	require.NoError(t, err)
	published, err := service.Publish(created.Page.ID, PublishInput{Notes: "control"})
	require.NoError(t, err)
	draft, err := service.UpdateDraft(created.Page.ID, PageDraftInput{
		Path: "/campaign", Title: "Campaign variant", Payload: PagePayload{"blocks": []any{}},
	})
	require.NoError(t, err)
	return published, draft
}

func TestDeliveryScheduleReconciliationIsIdempotent(t *testing.T) {
	db := newServiceTestDB(t)
	service := NewPageService(db)
	_, draft := createDeliveryTestPage(t, service)
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)

	_, err := service.UpdateDelivery(draft.Page.ID, DeliveryInput{
		Schedule: &ScheduleInput{PublishAt: now, UnpublishAt: &expires, Timezone: "UTC"},
	})
	require.NoError(t, err)

	summary, err := ReconcileDelivery(db, now)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Published)
	require.Equal(t, 0, summary.Unpublished)

	summary, err = ReconcileDelivery(db, now.Add(time.Minute))
	require.NoError(t, err)
	require.Zero(t, summary.Published)
	var publications int64
	require.NoError(t, db.Model(&models.CMSPublication{}).Where("entry_id = ?", draft.Entry.ID).Count(&publications).Error)
	require.EqualValues(t, 2, publications)

	summary, err = ReconcileDelivery(db, expires)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Unpublished)
	resolved, err := service.ResolvePublished("/campaign")
	require.Nil(t, resolved)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestDeliveryTargetingUsesDeterministicRuleMatching(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))
	_, draft := createDeliveryTestPage(t, service)
	_, err := service.UpdateDelivery(draft.Page.ID, DeliveryInput{
		TargetingRules: []TargetingRuleInput{{
			TargetingRule: TargetingRule{
				Markets: []string{"US"}, DeviceClasses: []string{"mobile"}, AuthStates: []string{"guest"},
				UTMSources: []string{"newsletter"},
			},
			IsEnabled: true,
		}},
	})
	require.NoError(t, err)
	published, err := service.Publish(draft.Page.ID, PublishInput{Notes: "targeted version"})
	require.NoError(t, err)

	matching := RequestContext{Market: "us", DeviceClass: "mobile", UTMSource: "newsletter", AssignmentKey: "visitor-1", CorrelationID: "request-1"}
	decision, eligible, err := service.ResolveDelivery(published, matching, time.Now())
	require.NoError(t, err)
	require.True(t, eligible)
	require.Equal(t, published.PublishedVersion.ID, decision.ContentVersionID)

	matching.Market = "ca"
	_, eligible, err = service.ResolveDelivery(published, matching, time.Now())
	require.NoError(t, err)
	require.False(t, eligible)
}

func TestDeliveryExperimentAllocationIsStickyAndWithinTolerance(t *testing.T) {
	service := NewPageService(newServiceTestDB(t))
	published, draft := createDeliveryTestPage(t, service)
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	_, err := service.UpdateDelivery(draft.Page.ID, DeliveryInput{
		Experiment: &ExperimentInput{
			Name: "Campaign hero", Status: models.CMSExperimentStatusActive, StickyKey: "visitor", StartsAt: now.Add(-time.Hour),
			Variants: []ExperimentVariantInput{
				{Name: "Control", VersionID: published.PublishedVersion.ID, Allocation: 7000},
				{Name: "Variant", VersionID: draft.CurrentVersion.ID, Allocation: 3000},
			},
		},
	})
	require.NoError(t, err)

	counts := map[uint]int{}
	for index := 0; index < 10000; index++ {
		request := RequestContext{AssignmentKey: fmt.Sprintf("visitor-%d", index), CorrelationID: fmt.Sprintf("request-%d", index)}
		decision, eligible, resolveErr := service.ResolveDelivery(published, request, now)
		require.NoError(t, resolveErr)
		require.True(t, eligible)
		counts[decision.ContentVersionID]++
		repeated, _, resolveErr := service.ResolveDelivery(published, request, now)
		require.NoError(t, resolveErr)
		require.Equal(t, decision.ContentVersionID, repeated.ContentVersionID)
	}
	require.InDelta(t, 7000, counts[published.PublishedVersion.ID], 200)
	require.InDelta(t, 3000, counts[draft.CurrentVersion.ID], 200)
}

func TestContentEventsAreValidatedAndDeduplicated(t *testing.T) {
	db := newServiceTestDB(t)
	service := NewPageService(db)
	published, _ := createDeliveryTestPage(t, service)
	input := ContentEventInput{
		ContentVersionID: published.PublishedVersion.ID, CorrelationID: "correlation-1", EventType: "impression",
	}
	require.NoError(t, service.RecordContentEvent(input))
	require.NoError(t, service.RecordContentEvent(input))
	var count int64
	require.NoError(t, db.Model(&models.CMSExposureEvent{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	input.EventType = "invalid"
	require.ErrorIs(t, service.RecordContentEvent(input), ErrInvalidDelivery)
}
