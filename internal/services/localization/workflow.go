package localization

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"ecommerce/internal/requestctx"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrLocalizationForbidden = errors.New("localization operation is forbidden")
	placeholderPattern       = regexp.MustCompile(`\{([a-zA-Z][a-zA-Z0-9_]*)\}`)
)

type ValueOptions struct {
	AssigneeID      *uint
	ExpectedVersion *uint
	ChangeSummary   string
}

type ValidationIssue struct {
	Path   string
	Code   string
	Detail string
}

type QueueFilter struct {
	Locale     string
	Namespace  string
	State      string
	AssigneeID *uint
	Missing    *bool
	Stale      *bool
	Query      string
	Page       int
	Limit      int
}

type QueueItem struct {
	Key              models.TranslationKey
	Locale           string
	LatestValue      *models.TranslationValue
	PublishedValue   *models.TranslationValue
	Missing          bool
	Stale            bool
	ValidationIssues []ValidationIssue
	PreviewURL       string
}

type QueuePage struct {
	Items      []QueueItem
	Page       int
	Limit      int
	Total      int
	TotalPages int
}

type ImportEntry struct {
	Key    string
	Status string
	Detail string
}

type ImportReport struct {
	DryRun    bool
	Created   int
	Unchanged int
	Conflicts int
	Invalid   int
	Entries   []ImportEntry
}

type ExportDocument struct {
	Locale    string
	Namespace string
	Format    string
	Filename  string
	Content   string
}

type Assignee struct {
	ID               uint
	Name             string
	Email            string
	LocalizationRole models.LocalizationRole
}

func sourceHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return fmt.Sprintf("%x", sum[:])
}

func (s *Service) CreateValueWithOptions(ctx context.Context, keyID uint, localeCode, value string, actorID *uint, options ValueOptions) (models.TranslationValue, error) {
	value = strings.TrimSpace(value)
	if keyID == 0 || value == "" {
		return models.TranslationValue{}, ErrInvalidTranslation
	}
	locale, err := s.RequireEnabledLocale(ctx, localeCode)
	if err != nil {
		return models.TranslationValue{}, err
	}
	var result models.TranslationValue
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if options.AssigneeID != nil {
			var eligibleCount int64
			if err := tx.Model(&models.User{}).Where("id = ? AND role = ?", *options.AssigneeID, "admin").Count(&eligibleCount).Error; err != nil {
				return err
			}
			if eligibleCount != 1 {
				return fmt.Errorf("%w: assignee does not have access to localization controls", ErrInvalidTranslation)
			}
		}
		var key models.TranslationKey
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&key, keyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTranslationKeyNotFound
			}
			return err
		}
		var maxVersion uint
		if err := tx.Model(&models.TranslationValue{}).Where("translation_key_id = ? AND locale_id = ?", keyID, locale.ID).Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error; err != nil {
			return err
		}
		if options.ExpectedVersion != nil && maxVersion != *options.ExpectedVersion {
			return fmt.Errorf("%w: expected version %d, found %d", ErrInvalidTransition, *options.ExpectedVersion, maxVersion)
		}
		result = models.TranslationValue{TranslationKeyID: keyID, LocaleID: locale.ID, Value: value, State: models.TranslationStateDraft, Version: maxVersion + 1, UpdatedBy: actorID, AssigneeID: options.AssigneeID, SourceHash: sourceHash(key.SourceText), ChangeSummary: strings.TrimSpace(options.ChangeSummary)}
		if err := tx.Select("*").Create(&result).Error; err != nil {
			return err
		}
		return createAudit(tx, ctx, &key, &result, nil, "draft_created", locale.Code, result.ChangeSummary, actorID)
	})
	return result, err
}

func (s *Service) ValidateValue(ctx context.Context, valueID uint) ([]ValidationIssue, error) {
	var value models.TranslationValue
	if err := s.db.WithContext(ctx).Preload("TranslationKey").Preload("Locale").First(&value, valueID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTranslationValueNotFound
		}
		return nil, err
	}
	return s.validateValueRecord(ctx, value)
}

func (s *Service) validateValueRecord(ctx context.Context, value models.TranslationValue) ([]ValidationIssue, error) {
	issues := comparePlaceholders(value.TranslationKey.SourceText, value.Value)
	var terms []models.LocalizationGlossaryTerm
	if err := s.db.WithContext(ctx).Where("locale_id = ? AND is_locked = ?", value.LocaleID, true).Find(&terms).Error; err != nil {
		return nil, err
	}
	lowerSource, lowerValue := strings.ToLower(value.TranslationKey.SourceText), strings.ToLower(value.Value)
	for _, term := range terms {
		if strings.Contains(lowerSource, strings.ToLower(term.SourceTerm)) && !strings.Contains(lowerValue, strings.ToLower(term.TranslatedTerm)) {
			issues = append(issues, ValidationIssue{Path: "/value", Code: "glossary_term_locked", Detail: fmt.Sprintf("Use the locked translation %q for %q.", term.TranslatedTerm, term.SourceTerm)})
		}
	}
	if pluralSiblingKey(value.TranslationKey.Key) != "" {
		var sibling models.TranslationKey
		if err := s.db.WithContext(ctx).Where("namespace = ? AND key = ? AND deleted_at IS NULL", value.TranslationKey.Namespace, pluralSiblingKey(value.TranslationKey.Key)).First(&sibling).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			issues = append(issues, ValidationIssue{Path: "/value", Code: "plural_sibling_missing", Detail: "The plural key requires a matching other form."})
		} else if err != nil {
			return nil, err
		} else {
			var count int64
			if err := s.db.WithContext(ctx).Model(&models.TranslationValue{}).Where("translation_key_id = ? AND locale_id = ? AND deleted_at IS NULL", sibling.ID, value.LocaleID).Count(&count).Error; err != nil {
				return nil, err
			}
			if count == 0 {
				issues = append(issues, ValidationIssue{Path: "/value", Code: "plural_translation_missing", Detail: "Translate the matching other plural form before review."})
			}
		}
	}
	return issues, nil
}

func comparePlaceholders(source, translated string) []ValidationIssue {
	collect := func(value string) map[string]int {
		result := map[string]int{}
		for _, match := range placeholderPattern.FindAllStringSubmatch(value, -1) {
			result[match[1]]++
		}
		return result
	}
	expected, actual := collect(source), collect(translated)
	if maps.Equal(expected, actual) {
		return nil
	}
	return []ValidationIssue{{Path: "/value", Code: "placeholder_mismatch", Detail: "Translated placeholders must exactly match the source placeholders."}}
}

func pluralSiblingKey(key string) string {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return ""
	}
	switch parts[len(parts)-1] {
	case "zero", "one", "two", "few", "many":
		parts[len(parts)-1] = "other"
		return strings.Join(parts, ".")
	default:
		return ""
	}
}

func (s *Service) TransitionValueWithAudit(ctx context.Context, valueID uint, target models.TranslationState, actorID *uint, summary string) (models.TranslationValue, error) {
	if target == models.TranslationStateReview || target == models.TranslationStatePublished {
		issues, err := s.ValidateValue(ctx, valueID)
		if err != nil {
			return models.TranslationValue{}, err
		}
		if len(issues) != 0 {
			return models.TranslationValue{}, fmt.Errorf("%w: %s", ErrInvalidTranslation, issues[0].Detail)
		}
	}
	var result models.TranslationValue
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("TranslationKey").Preload("Locale").First(&result, valueID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTranslationValueNotFound
			}
			return err
		}
		valid := result.State == models.TranslationStateDraft && target == models.TranslationStateReview || result.State == models.TranslationStateReview && target == models.TranslationStatePublished
		if !valid {
			return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, result.State, target)
		}
		result.State, result.UpdatedBy, result.ChangeSummary = target, actorID, strings.TrimSpace(summary)
		if target == models.TranslationStatePublished {
			result.ReviewedBy = actorID
		}
		if err := tx.Select("*").Save(&result).Error; err != nil {
			return err
		}
		return createAudit(tx, ctx, &result.TranslationKey, &result, nil, "value_"+string(target), result.Locale.Code, result.ChangeSummary, actorID)
	})
	if err == nil && target == models.TranslationStatePublished {
		latency := time.Since(result.CreatedAt).Milliseconds()
		if latency < 0 {
			latency = 0
		}
		s.recordMetricBestEffort(ctx, MetricPublishLatency, result.Locale.Code, result.TranslationKey.OwnerDomain, result.TranslationKey.Namespace+"."+result.TranslationKey.Key, 1, latency)
	}
	return result, err
}

func (s *Service) ListValues(ctx context.Context, keyID uint) ([]models.TranslationValue, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.TranslationKey{}).Where("id = ?", keyID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, ErrTranslationKeyNotFound
	}
	var values []models.TranslationValue
	err := s.db.WithContext(ctx).Preload("Locale").Where("translation_key_id = ?", keyID).Order("locale_id ASC, version DESC").Find(&values).Error
	return values, err
}

func (s *Service) ListQueue(ctx context.Context, filter QueueFilter) (QueuePage, error) {
	page := filter.Page
	if page == 0 {
		page = 1
	}
	limit := filter.Limit
	if limit == 0 {
		limit = 25
	}
	if page < 1 || limit < 1 || limit > 100 {
		return QueuePage{}, ErrInvalidTranslation
	}
	locale, err := s.RequireEnabledLocale(ctx, filter.Locale)
	if err != nil {
		return QueuePage{}, err
	}
	query := s.db.WithContext(ctx).Model(&models.TranslationKey{}).Where("is_deprecated = ?", false)
	if filter.Namespace != "" {
		if !validNamespace(filter.Namespace) {
			return QueuePage{}, ErrInvalidTranslation
		}
		query = query.Where("namespace = ?", filter.Namespace)
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		search := "LOWER(key) LIKE ? OR LOWER(namespace) LIKE ? OR LOWER(source_text) LIKE ? OR LOWER(description) LIKE ?"
		args := []any{like, like, like, like}
		if separator := strings.IndexByte(q, '.'); separator > 0 && separator < len(q)-1 {
			namespaceQuery := strings.ToLower(strings.TrimSpace(q[:separator]))
			keyQuery := "%" + strings.ToLower(strings.TrimSpace(q[separator+1:])) + "%"
			if validNamespace(namespaceQuery) {
				search += " OR (LOWER(namespace) = ? AND LOWER(key) LIKE ?)"
				args = append(args, namespaceQuery, keyQuery)
			}
		}
		query = query.Where(search, args...)
	}
	requiresPostFiltering := filter.State != "" || filter.AssigneeID != nil || filter.Missing != nil || filter.Stale != nil
	total := 0
	totalPages := 0
	if !requiresPostFiltering {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return QueuePage{}, err
		}
		total = int(count)
		if total != 0 {
			totalPages = (total + limit - 1) / limit
		}
		query = query.Offset((page - 1) * limit).Limit(limit)
	}
	var keys []models.TranslationKey
	if err := query.Order("namespace ASC, key ASC").Find(&keys).Error; err != nil {
		return QueuePage{}, err
	}
	keyIDs := make([]uint, 0, len(keys))
	for _, key := range keys {
		keyIDs = append(keyIDs, key.ID)
	}
	valuesByKey := make(map[uint][]models.TranslationValue, len(keys))
	if len(keyIDs) != 0 {
		var values []models.TranslationValue
		if err := s.db.WithContext(ctx).
			Preload("TranslationKey").
			Preload("Locale").
			Where("translation_key_id IN ? AND locale_id = ?", keyIDs, locale.ID).
			Order("translation_key_id ASC, version DESC").
			Find(&values).Error; err != nil {
			return QueuePage{}, err
		}
		for _, value := range values {
			valuesByKey[value.TranslationKeyID] = append(valuesByKey[value.TranslationKeyID], value)
		}
	}
	items := make([]QueueItem, 0, len(keys))
	for _, key := range keys {
		values := valuesByKey[key.ID]
		item := QueueItem{Key: key, Locale: locale.Code, Missing: len(values) == 0, PreviewURL: previewURL(key, locale.Code)}
		if len(values) != 0 {
			latest := values[0]
			item.LatestValue = &latest
			item.Stale = latest.SourceHash != sourceHash(key.SourceText)
			item.ValidationIssues, err = s.validateValueRecord(ctx, latest)
			if err != nil {
				return QueuePage{}, err
			}
			for _, value := range values {
				if value.State == models.TranslationStatePublished {
					copy := value
					item.PublishedValue = &copy
					break
				}
			}
		}
		if filter.State != "" && (item.LatestValue == nil || string(item.LatestValue.State) != filter.State) {
			continue
		}
		if filter.AssigneeID != nil && (item.LatestValue == nil || item.LatestValue.AssigneeID == nil || *item.LatestValue.AssigneeID != *filter.AssigneeID) {
			continue
		}
		if filter.Missing != nil && item.Missing != *filter.Missing {
			continue
		}
		if filter.Stale != nil && item.Stale != *filter.Stale {
			continue
		}
		items = append(items, item)
	}
	if requiresPostFiltering {
		total = len(items)
		if total != 0 {
			totalPages = (total + limit - 1) / limit
		}
		start := (page - 1) * limit
		if start >= total {
			items = []QueueItem{}
		} else {
			end := min(start+limit, total)
			items = items[start:end]
		}
	}
	return QueuePage{Items: items, Page: page, Limit: limit, Total: total, TotalPages: totalPages}, nil
}

func previewURL(key models.TranslationKey, locale string) string {
	path := "/"
	if key.Namespace == "admin" {
		path = "/admin/localization"
	}
	return path + "?locale=" + url.QueryEscape(locale)
}

func (s *Service) AddComment(ctx context.Context, valueID uint, comment, subject string, actorID *uint) (models.TranslationComment, error) {
	comment = strings.TrimSpace(comment)
	if comment == "" || len(comment) > 4000 {
		return models.TranslationComment{}, ErrInvalidTranslation
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.TranslationValue{}).Where("id = ?", valueID).Count(&count).Error; err != nil {
		return models.TranslationComment{}, err
	}
	if count == 0 {
		return models.TranslationComment{}, ErrTranslationValueNotFound
	}
	row := models.TranslationComment{TranslationValueID: valueID, AuthorID: actorID, AuthorSubject: subject, Comment: comment}
	err := s.db.WithContext(ctx).Select("*").Create(&row).Error
	if err == nil {
		row.AuthorName, err = s.commentAuthorName(ctx, row)
	}
	return row, err
}

func (s *Service) ListComments(ctx context.Context, valueID uint) ([]models.TranslationComment, error) {
	var rows []models.TranslationComment
	if err := s.db.WithContext(ctx).Where("translation_value_id = ?", valueID).Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for index := range rows {
		name, err := s.commentAuthorName(ctx, rows[index])
		if err != nil {
			return nil, err
		}
		rows[index].AuthorName = name
	}
	return rows, nil
}

func (s *Service) commentAuthorName(ctx context.Context, comment models.TranslationComment) (string, error) {
	var user models.User
	query := s.db.WithContext(ctx)
	if comment.AuthorID != nil {
		query = query.Where("id = ?", *comment.AuthorID)
	} else if strings.TrimSpace(comment.AuthorSubject) != "" {
		query = query.Where("subject = ?", comment.AuthorSubject)
	} else {
		return "Administrator", nil
	}
	if err := query.First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "Administrator", nil
		}
		return "", err
	}
	for _, candidate := range []string{user.Name, user.Username, user.Email} {
		if name := strings.TrimSpace(candidate); name != "" {
			return name, nil
		}
	}
	return "Administrator", nil
}

func (s *Service) ListReleases(ctx context.Context) ([]models.TranslationRelease, error) {
	var rows []models.TranslationRelease
	err := s.db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error
	return rows, err
}

func (s *Service) ListGlossary(ctx context.Context, localeCode string) ([]models.LocalizationGlossaryTerm, error) {
	query := s.db.WithContext(ctx).Preload("Locale")
	if strings.TrimSpace(localeCode) != "" {
		locale, err := s.RequireEnabledLocale(ctx, localeCode)
		if err != nil {
			return nil, err
		}
		query = query.Where("locale_id = ?", locale.ID)
	}
	var rows []models.LocalizationGlossaryTerm
	err := query.Order("locale_id ASC, source_term ASC").Find(&rows).Error
	return rows, err
}

func (s *Service) PutGlossary(ctx context.Context, localeCode, sourceTerm, translatedTerm, description string, locked bool) (models.LocalizationGlossaryTerm, error) {
	locale, err := s.RequireEnabledLocale(ctx, localeCode)
	if err != nil {
		return models.LocalizationGlossaryTerm{}, err
	}
	sourceTerm, translatedTerm = strings.TrimSpace(sourceTerm), strings.TrimSpace(translatedTerm)
	if sourceTerm == "" || translatedTerm == "" {
		return models.LocalizationGlossaryTerm{}, ErrInvalidTranslation
	}
	var row models.LocalizationGlossaryTerm
	err = s.db.WithContext(ctx).Where("locale_id = ? AND source_term = ?", locale.ID, sourceTerm).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.LocalizationGlossaryTerm{LocaleID: locale.ID, SourceTerm: sourceTerm}
		err = nil
	}
	if err != nil {
		return row, err
	}
	row.TranslatedTerm, row.Description, row.IsLocked = translatedTerm, strings.TrimSpace(description), locked
	if row.ID == 0 {
		err = s.db.WithContext(ctx).Select("*").Create(&row).Error
	} else {
		err = s.db.WithContext(ctx).Select("*").Save(&row).Error
	}
	row.Locale = models.Locale{BaseModel: models.BaseModel{ID: locale.ID}, Code: locale.Code}
	return row, err
}

func (s *Service) DeleteGlossary(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.LocalizationGlossaryTerm{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTranslationValueNotFound
	}
	return nil
}

func validLocalizationRole(role models.LocalizationRole) bool {
	return role == models.LocalizationRoleTranslator || role == models.LocalizationRoleEditor || role == models.LocalizationRolePublisher
}

func (s *Service) PutRole(ctx context.Context, subject string, role models.LocalizationRole) (models.LocalizationRoleAssignment, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" || !validLocalizationRole(role) {
		return models.LocalizationRoleAssignment{}, ErrInvalidTranslation
	}
	var row models.LocalizationRoleAssignment
	err := s.db.WithContext(ctx).Where("subject = ?", subject).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.LocalizationRoleAssignment{Subject: subject}
		err = nil
	}
	if err != nil {
		return row, err
	}
	row.Role = role
	if row.ID == 0 {
		err = s.db.WithContext(ctx).Select("*").Create(&row).Error
	} else {
		err = s.db.WithContext(ctx).Select("*").Save(&row).Error
	}
	return row, err
}

func (s *Service) ListRoles(ctx context.Context) ([]models.LocalizationRoleAssignment, error) {
	var rows []models.LocalizationRoleAssignment
	err := s.db.WithContext(ctx).Order("subject ASC").Find(&rows).Error
	return rows, err
}

func (s *Service) ListAssignees(ctx context.Context, search string, limit int) ([]Assignee, error) {
	if limit < 1 || limit > 50 {
		return nil, ErrInvalidTranslation
	}
	type assigneeRow struct {
		ID               uint
		Name             string
		Username         string
		Email            string
		LocalizationRole string
	}
	query := s.db.WithContext(ctx).Table("users AS users").
		Select("users.id, users.name, users.username, users.email, COALESCE(localization_roles.role, 'publisher') AS localization_role").
		Joins("LEFT JOIN localization_role_assignments AS localization_roles ON localization_roles.subject = users.subject AND localization_roles.deleted_at IS NULL").
		Where("users.deleted_at IS NULL AND users.role = ?", "admin")
	if value := strings.TrimSpace(search); value != "" {
		like := "%" + strings.ToLower(value) + "%"
		query = query.Where("CAST(users.id AS TEXT) = ? OR LOWER(users.name) LIKE ? OR LOWER(users.username) LIKE ? OR LOWER(users.email) LIKE ?", value, like, like, like)
	}
	var rows []assigneeRow
	if err := query.Order("LOWER(COALESCE(NULLIF(users.name, ''), users.username, users.email)) ASC, users.id ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]Assignee, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" {
			name = strings.TrimSpace(row.Username)
		}
		if name == "" {
			name = strings.TrimSpace(row.Email)
		}
		result = append(result, Assignee{ID: row.ID, Name: name, Email: row.Email, LocalizationRole: models.LocalizationRole(row.LocalizationRole)})
	}
	return result, nil
}

func (s *Service) RequireRole(ctx context.Context, minimum models.LocalizationRole) error {
	principal, ok := requestctx.PrincipalFrom(ctx)
	if !ok || !principal.HasRole("admin") {
		return ErrLocalizationForbidden
	}
	var assignment models.LocalizationRoleAssignment
	err := s.db.WithContext(ctx).Where("subject = ?", principal.Subject).First(&assignment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		assignment.Role = models.LocalizationRolePublisher
	} else if err != nil {
		return err
	}
	rank := map[models.LocalizationRole]int{models.LocalizationRoleTranslator: 1, models.LocalizationRoleEditor: 2, models.LocalizationRolePublisher: 3}
	if rank[assignment.Role] < rank[minimum] {
		return ErrLocalizationForbidden
	}
	return nil
}

func createAudit(tx *gorm.DB, ctx context.Context, key *models.TranslationKey, value *models.TranslationValue, release *models.TranslationRelease, action, locale, summary string, actorID *uint) error {
	principal, _ := requestctx.PrincipalFrom(ctx)
	row := models.TranslationAuditEvent{ActorID: actorID, ActorSubject: principal.Subject, Action: action, Locale: locale, ChangeSummary: strings.TrimSpace(summary)}
	if key != nil {
		row.TranslationKeyID, row.Namespace = &key.ID, key.Namespace
	}
	if value != nil {
		row.TranslationValueID = &value.ID
	}
	if release != nil {
		row.ReleaseID = &release.ID
	}
	return tx.Select("*").Create(&row).Error
}

func (s *Service) CreateReleaseWithAudit(ctx context.Context, name, notes string, actorID *uint) (models.TranslationRelease, error) {
	release, err := s.CreateRelease(ctx, name, notes)
	if err != nil {
		return release, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createAudit(tx, ctx, nil, nil, &release, "release_created", "", notes, actorID)
	})
	return release, err
}

func (s *Service) ActivateReleaseWithAudit(ctx context.Context, releaseID uint, actorID *uint) (models.TranslationRelease, error) {
	var target models.TranslationRelease
	if err := s.db.WithContext(ctx).Select("snapshot_hash").First(&target, releaseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.TranslationRelease{}, ErrReleaseNotFound
		}
		return models.TranslationRelease{}, err
	}
	var priorSnapshotCount int64
	if err := s.db.WithContext(ctx).Model(&models.TranslationRelease{}).
		Where("status = ? AND snapshot_hash = ?", models.TranslationReleaseStatusSuperseded, target.SnapshotHash).
		Count(&priorSnapshotCount).Error; err != nil {
		return models.TranslationRelease{}, err
	}
	release, err := s.ActivateRelease(ctx, releaseID, actorID)
	if err != nil {
		return release, err
	}
	action := "release_activated"
	if priorSnapshotCount > 0 {
		action = "release_rollback_activated"
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createAudit(tx, ctx, nil, nil, &release, action, "", release.Notes, actorID)
	})
	if err == nil && priorSnapshotCount > 0 {
		s.recordMetricBestEffort(ctx, MetricRollback, "", "admin", release.Name, 1, 0)
	}
	return release, err
}

func (s *Service) RollbackReleaseWithAudit(ctx context.Context, sourceReleaseID uint, actorID *uint) (models.TranslationRelease, error) {
	var result models.TranslationRelease
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source models.TranslationRelease
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&source, sourceReleaseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReleaseNotFound
			}
			return err
		}
		if source.Status != models.TranslationReleaseStatusSuperseded {
			return fmt.Errorf("%w: only a superseded release can be restored", ErrInvalidTransition)
		}
		quality, err := validateReleaseQuality(tx, source.ID)
		if err != nil {
			return err
		}
		if !quality.Ready {
			return fmt.Errorf("%w: %d critical translations are missing", ErrReleaseQualityFailed, len(quality.Missing))
		}
		var entries []models.TranslationReleaseEntry
		if err := tx.Where("release_id = ?", source.ID).Order("id ASC").Find(&entries).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.TranslationRelease{}).
			Where("status = ?", models.TranslationReleaseStatusActive).
			Update("status", models.TranslationReleaseStatusSuperseded).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		name := "Rollback to " + source.Name
		if len(name) > 128 {
			name = name[:128]
		}
		result = models.TranslationRelease{
			Name: name, Status: models.TranslationReleaseStatusActive,
			PublishedAt: &now, PublishedBy: actorID,
			Notes: fmt.Sprintf("Restored immutable snapshot from release #%d.", source.ID), SnapshotHash: source.SnapshotHash,
		}
		if err := tx.Select("*").Create(&result).Error; err != nil {
			return err
		}
		for _, entry := range entries {
			copy := models.TranslationReleaseEntry{ReleaseID: result.ID, TranslationValueID: entry.TranslationValueID}
			if err := tx.Select("*").Create(&copy).Error; err != nil {
				return err
			}
		}
		return createAudit(tx, ctx, nil, nil, &result, "release_rollback_activated", "", result.Notes, actorID)
	})
	if err == nil {
		s.invalidateBundleCache(result.ID)
		s.recordMetricBestEffort(ctx, MetricRollback, "", "admin", result.Name, 1, 0)
	}
	return result, err
}

type translationDocumentEntry struct {
	Key    string `json:"key"`
	Source string `json:"source"`
	Value  string `json:"value"`
	State  string `json:"state"`
}

type xliffDocument struct {
	XMLName xml.Name  `xml:"xliff"`
	Version string    `xml:"version,attr"`
	File    xliffFile `xml:"file"`
}

type xliffFile struct {
	SourceLanguage string      `xml:"srcLang,attr"`
	TargetLanguage string      `xml:"trgLang,attr"`
	Units          []xliffUnit `xml:"unit"`
}

type xliffUnit struct {
	ID      string `xml:"id,attr"`
	Segment struct {
		Source string `xml:"source"`
		Target string `xml:"target"`
	} `xml:"segment"`
}

func (s *Service) Export(ctx context.Context, localeCode, namespace, format string) (ExportDocument, error) {
	locale, err := s.RequireEnabledLocale(ctx, localeCode)
	if err != nil {
		return ExportDocument{}, err
	}
	if namespace != "" && !validNamespace(namespace) {
		return ExportDocument{}, ErrInvalidTranslation
	}
	if format != "json" && format != "csv" && format != "xliff" {
		return ExportDocument{}, ErrInvalidTranslation
	}
	query := s.db.WithContext(ctx).Model(&models.TranslationKey{}).Where("is_deprecated = ?", false)
	if namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}
	var keys []models.TranslationKey
	if err := query.Order("namespace ASC, key ASC").Find(&keys).Error; err != nil {
		return ExportDocument{}, err
	}
	entries := make([]translationDocumentEntry, 0, len(keys))
	for _, key := range keys {
		var value models.TranslationValue
		err := s.db.WithContext(ctx).Where("translation_key_id = ? AND locale_id = ?", key.ID, locale.ID).Order("version DESC").First(&value).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return ExportDocument{}, err
		}
		entries = append(entries, translationDocumentEntry{Key: key.Namespace + "." + key.Key, Source: key.SourceText, Value: value.Value, State: string(value.State)})
	}
	var content string
	switch format {
	case "json":
		data := map[string]string{}
		for _, entry := range entries {
			data[entry.Key] = entry.Value
		}
		raw, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return ExportDocument{}, err
		}
		content = string(raw) + "\n"
	case "csv":
		var buffer bytes.Buffer
		writer := csv.NewWriter(&buffer)
		_ = writer.Write([]string{"key", "source", "value", "state"})
		for _, entry := range entries {
			_ = writer.Write([]string{entry.Key, entry.Source, entry.Value, entry.State})
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return ExportDocument{}, err
		}
		content = buffer.String()
	case "xliff":
		doc := xliffDocument{Version: "2.0", File: xliffFile{SourceLanguage: "en-US", TargetLanguage: locale.Code}}
		for _, entry := range entries {
			unit := xliffUnit{ID: entry.Key}
			unit.Segment.Source, unit.Segment.Target = entry.Source, entry.Value
			doc.File.Units = append(doc.File.Units, unit)
		}
		raw, err := xml.MarshalIndent(doc, "", "  ")
		if err != nil {
			return ExportDocument{}, err
		}
		content = xml.Header + string(raw) + "\n"
	}
	name := "translations-" + locale.Code
	if namespace != "" {
		name += "-" + namespace
	}
	return ExportDocument{Locale: locale.Code, Namespace: namespace, Format: format, Filename: name + "." + format, Content: content}, nil
}

func parseImportDocument(format, content string) (map[string]string, error) {
	result := map[string]string{}
	switch format {
	case "json":
		if err := json.Unmarshal([]byte(content), &result); err != nil {
			return nil, ErrInvalidTranslation
		}
	case "csv":
		rows, err := csv.NewReader(strings.NewReader(content)).ReadAll()
		if err != nil || len(rows) == 0 {
			return nil, ErrInvalidTranslation
		}
		keyIndex, valueIndex := -1, -1
		for index, heading := range rows[0] {
			if heading == "key" {
				keyIndex = index
			}
			if heading == "value" {
				valueIndex = index
			}
		}
		if keyIndex < 0 || valueIndex < 0 {
			return nil, ErrInvalidTranslation
		}
		for _, row := range rows[1:] {
			if keyIndex < len(row) && valueIndex < len(row) {
				result[row[keyIndex]] = row[valueIndex]
			}
		}
	case "xliff":
		var doc xliffDocument
		if err := xml.Unmarshal([]byte(content), &doc); err != nil {
			return nil, ErrInvalidTranslation
		}
		for _, unit := range doc.File.Units {
			result[unit.ID] = unit.Segment.Target
		}
	default:
		return nil, ErrInvalidTranslation
	}
	return result, nil
}

func (s *Service) Import(ctx context.Context, localeCode, namespace, format, content string, dryRun bool, actorID *uint) (ImportReport, error) {
	locale, err := s.RequireEnabledLocale(ctx, localeCode)
	if err != nil {
		return ImportReport{}, err
	}
	if namespace != "" && !validNamespace(namespace) {
		return ImportReport{}, ErrInvalidTranslation
	}
	document, err := parseImportDocument(format, content)
	if err != nil {
		return ImportReport{}, err
	}
	keys := make([]string, 0, len(document))
	for key := range document {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	report := ImportReport{DryRun: dryRun}
	for _, qualified := range keys {
		entry := ImportEntry{Key: qualified}
		separator := strings.IndexByte(qualified, '.')
		if separator <= 0 || (namespace != "" && qualified[:separator] != namespace) || strings.TrimSpace(document[qualified]) == "" {
			entry.Status, entry.Detail, report.Invalid = "invalid", "Key or value is invalid.", report.Invalid+1
			report.Entries = append(report.Entries, entry)
			continue
		}
		var key models.TranslationKey
		if err := s.db.WithContext(ctx).Where("namespace = ? AND key = ? AND is_deprecated = ?", qualified[:separator], qualified[separator+1:], false).First(&key).Error; err != nil {
			entry.Status, entry.Detail, report.Invalid = "invalid", "Translation key does not exist.", report.Invalid+1
			report.Entries = append(report.Entries, entry)
			continue
		}
		var latest models.TranslationValue
		err := s.db.WithContext(ctx).Where("translation_key_id = ? AND locale_id = ?", key.ID, locale.ID).Order("version DESC").First(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return report, err
		}
		if latest.Value == document[qualified] {
			entry.Status, entry.Detail, report.Unchanged = "unchanged", "Value is already current.", report.Unchanged+1
		} else if latest.ID != 0 && latest.State != models.TranslationStatePublished {
			entry.Status, entry.Detail, report.Conflicts = "conflict", "An unpublished translation version already exists.", report.Conflicts+1
		} else {
			entry.Status, entry.Detail, report.Created = "created", "Draft translation version created.", report.Created+1
			if !dryRun {
				if _, err := s.CreateValueWithOptions(ctx, key.ID, locale.Code, document[qualified], actorID, ValueOptions{ChangeSummary: "Imported from " + format}); err != nil {
					return report, err
				}
			}
		}
		report.Entries = append(report.Entries, entry)
	}
	return report, nil
}
