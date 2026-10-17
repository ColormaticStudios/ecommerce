package search

import (
	"context"
	"ecommerce/models"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"reflect"
)

// auditConfigurationChanges records every affected row, including implicit
// default/active transitions, in the same transaction as the configuration.
func (s *Service) auditConfigurationChanges(tx *gorm.DB, kind string, actor *uint, mutate func() error) error {
	before, err := configurationSnapshots(tx, kind)
	if err != nil {
		return err
	}
	if err = mutate(); err != nil {
		return err
	}
	after, err := configurationSnapshots(tx, kind)
	if err != nil {
		return err
	}
	for id, old := range before {
		next, exists := after[id]
		if exists && reflect.DeepEqual(old, next) {
			delete(after, id)
			continue
		}
		operation := "update"
		var newJSON *string
		if exists {
			raw := string(next)
			newJSON = &raw
			delete(after, id)
		} else {
			operation = "delete"
		}
		oldJSON := string(old)
		if err := tx.Create(&models.SearchConfigurationAudit{Kind: kind, EntityID: id, Operation: operation, ActorID: actor, BeforeJSON: &oldJSON, AfterJSON: newJSON, CreatedAt: s.now().UTC()}).Error; err != nil {
			return err
		}
	}
	for id, next := range after {
		raw := string(next)
		if err := tx.Create(&models.SearchConfigurationAudit{Kind: kind, EntityID: id, Operation: "create", ActorID: actor, AfterJSON: &raw, CreatedAt: s.now().UTC()}).Error; err != nil {
			return err
		}
	}
	return nil
}
func configurationSnapshots(tx *gorm.DB, kind string) (map[uint]json.RawMessage, error) {
	result := map[uint]json.RawMessage{}
	add := func(id uint, value any) error { raw, err := json.Marshal(value); result[id] = raw; return err }
	switch kind {
	case "synonym":
		var rows []models.SearchSynonymSet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("id").Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			value, err := synonymSetFromModel(row)
			if err != nil {
				return nil, err
			}
			if err = add(row.ID, value); err != nil {
				return nil, err
			}
		}
	case "typo_profile":
		var rows []models.SearchTypoToleranceProfile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("id").Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			if err := add(row.ID, row); err != nil {
				return nil, err
			}
		}
	case "ranking_profile":
		var rows []models.SearchRankingProfile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("id").Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			value, err := rankingProfileFromModel(row)
			if err != nil {
				return nil, err
			}
			if err = add(row.ID, value); err != nil {
				return nil, err
			}
		}
	default:
		return nil, ErrConfigurationInvalid
	}
	return result, nil
}
func (s *Service) ListConfigurationHistory(ctx context.Context, kind string, entityID uint, page, limit int) ([]models.SearchConfigurationAudit, int64, error) {
	if page < 1 || page > 1000000 || limit < 1 || limit > 100 || (kind != "" && kind != "synonym" && kind != "typo_profile" && kind != "ranking_profile") {
		return nil, 0, ErrConfigurationInvalid
	}
	if entityID > 0 && kind == "" {
		return nil, 0, errors.New("configuration kind is required with an entity ID")
	}
	db, err := s.configurationDB(ctx)
	if err != nil {
		return nil, 0, err
	}
	q := db.Model(&models.SearchConfigurationAudit{})
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if entityID > 0 {
		q = q.Where("entity_id = ?", entityID)
	}
	var total int64
	if err = q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.SearchConfigurationAudit
	err = q.Order("created_at DESC, id DESC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error
	return rows, total, err
}
