package repository

import (
	"context"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"gorm.io/gorm"
)

// ComplianceDecisionRepository owns all persistence operations for 合规决定.
type ComplianceDecisionRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.ComplianceDecision], error)
	Get(context.Context, uint) (model.ComplianceDecision, error)
	ListInFlightByRelatedCode(context.Context, string) ([]model.ComplianceDecision, error)
	CreateWithRevision(context.Context, *model.ComplianceDecision, *model.DecisionRevision) error
	UpdateWithRevision(context.Context, uint, uint, *model.ComplianceDecision, *model.DecisionRevision) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type complianceDecisionRepository struct {
	store *Store[model.ComplianceDecision]
	db    *gorm.DB
}

func NewComplianceDecisionRepository(db *gorm.DB) ComplianceDecisionRepository {
	return &complianceDecisionRepository{store: NewStore[model.ComplianceDecision](db), db: db}
}

func (r *complianceDecisionRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.ComplianceDecision], error) {
	page, err := r.store.List(ctx, q)
	if err != nil || len(page.Items) == 0 {
		return page, err
	}
	ids := make([]uint, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.ID)
	}
	var revisions []model.DecisionRevision
	if err := r.db.WithContext(ctx).Where("compliance_decision_id IN ?", ids).
		Order("version ASC").Find(&revisions).Error; err != nil {
		return Page[model.ComplianceDecision]{}, err
	}
	byDecision := make(map[uint][]model.DecisionRevision)
	for _, revision := range revisions {
		byDecision[revision.ComplianceDecisionID] = append(byDecision[revision.ComplianceDecisionID], revision)
	}
	for index := range page.Items {
		page.Items[index].Revisions = byDecision[page.Items[index].ID]
	}
	return page, nil
}
func (r *complianceDecisionRepository) Get(ctx context.Context, id uint) (model.ComplianceDecision, error) {
	var item model.ComplianceDecision
	err := r.db.WithContext(ctx).Preload("Revisions", func(db *gorm.DB) *gorm.DB {
		return db.Order("version ASC")
	}).First(&item, id).Error
	return item, err
}
func (r *complianceDecisionRepository) ListInFlightByRelatedCode(ctx context.Context, relatedCode string) ([]model.ComplianceDecision, error) {
	items := make([]model.ComplianceDecision, 0)
	err := r.db.WithContext(ctx).
		Where("related_code = ? AND status IN ?", relatedCode, []string{"draft", "review"}).
		Order("updated_at DESC, id DESC").Find(&items).Error
	return items, err
}
func (r *complianceDecisionRepository) CreateWithRevision(ctx context.Context, item *model.ComplianceDecision, revision *model.DecisionRevision) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Revisions").Create(item).Error; err != nil {
			return err
		}
		revision.ComplianceDecisionID = item.ID
		return tx.Create(revision).Error
	})
}
func (r *complianceDecisionRepository) UpdateWithRevision(ctx context.Context, id, version uint, item *model.ComplianceDecision, revision *model.DecisionRevision) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.ComplianceDecision{}).Where("id = ? AND version = ?", id, version).
			Select("*").Omit("id", "code", "created_at", "deleted_at", "Revisions").Updates(item)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrVersionConflict
		}
		revision.ComplianceDecisionID = id
		return tx.Create(revision).Error
	})
}
func (r *complianceDecisionRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *complianceDecisionRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
