package repository

import (
	"context"

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
