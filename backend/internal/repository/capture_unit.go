package repository

import (
	"context"
	"strings"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"gorm.io/gorm"
)

// CaptureUnitRepository owns all persistence operations for 捕集装置.
type CaptureUnitRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.CaptureUnit], error)
	Get(context.Context, uint) (model.CaptureUnit, error)
	Create(context.Context, *model.CaptureUnit) error
	Update(context.Context, uint, uint, *model.CaptureUnit) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
	GetByRelatedCode(context.Context, *gorm.DB, string) (model.CaptureUnit, error)
}

type captureUnitRepository struct {
	store *Store[model.CaptureUnit]
}

func NewCaptureUnitRepository(db *gorm.DB) CaptureUnitRepository {
	return &captureUnitRepository{store: NewStore[model.CaptureUnit](db)}
}

func (r *captureUnitRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.CaptureUnit], error) {
	return r.store.List(ctx, q)
}
func (r *captureUnitRepository) Get(ctx context.Context, id uint) (model.CaptureUnit, error) {
	return r.store.Get(ctx, id)
}
func (r *captureUnitRepository) Create(ctx context.Context, item *model.CaptureUnit) error {
	return r.store.Create(ctx, item)
}
func (r *captureUnitRepository) Update(ctx context.Context, id, version uint, item *model.CaptureUnit) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *captureUnitRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *captureUnitRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}

// GetByRelatedCode resolves the capture unit behind a unit relation code on
// the caller's transaction, anchoring the per-unit retire verification.
func (r *captureUnitRepository) GetByRelatedCode(ctx context.Context, tx *gorm.DB, relatedCode string) (model.CaptureUnit, error) {
	var unit model.CaptureUnit
	err := tx.WithContext(ctx).
		Where("related_code = ?", strings.TrimSpace(relatedCode)).
		Order("id ASC").First(&unit).Error
	return unit, err
}
