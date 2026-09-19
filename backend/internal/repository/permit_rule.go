package repository

import (
	"context"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"gorm.io/gorm"
)

// PermitRuleRepository owns all persistence operations for 许可规则.
type PermitRuleRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.PermitRule], error)
	Get(context.Context, uint) (model.PermitRule, error)
	FindActiveByRelatedCode(context.Context, string) (model.PermitRule, error)
	Create(context.Context, *model.PermitRule) error
	Update(context.Context, uint, uint, *model.PermitRule) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type permitRuleRepository struct {
	store *Store[model.PermitRule]
}

func NewPermitRuleRepository(db *gorm.DB) PermitRuleRepository {
	return &permitRuleRepository{store: NewStore[model.PermitRule](db)}
}

func (r *permitRuleRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.PermitRule], error) {
	return r.store.List(ctx, q)
}
func (r *permitRuleRepository) Get(ctx context.Context, id uint) (model.PermitRule, error) {
	return r.store.Get(ctx, id)
}
func (r *permitRuleRepository) FindActiveByRelatedCode(ctx context.Context, relatedCode string) (model.PermitRule, error) {
	var item model.PermitRule
	err := r.store.db.WithContext(ctx).Where("related_code = ? AND status = ?", relatedCode, "active").
		Order("effective_at DESC, updated_at DESC, id DESC").First(&item).Error
	return item, err
}
func (r *permitRuleRepository) Create(ctx context.Context, item *model.PermitRule) error {
	return r.store.Create(ctx, item)
}
func (r *permitRuleRepository) Update(ctx context.Context, id, version uint, item *model.PermitRule) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *permitRuleRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *permitRuleRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
