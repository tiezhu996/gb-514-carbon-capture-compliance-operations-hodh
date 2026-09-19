package repository

import (
	"context"
	"strings"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"gorm.io/gorm"
)

// EmissionSampleRepository owns all persistence operations for 排放样本.
type EmissionSampleRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.EmissionSample], error)
	Get(context.Context, uint) (model.EmissionSample, error)
	Create(context.Context, *model.EmissionSample) error
	Update(context.Context, uint, uint, *model.EmissionSample) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
	LatestVerifiedByRelatedCode(context.Context, *gorm.DB, string) (model.EmissionSample, error)
}

type emissionSampleRepository struct {
	store *Store[model.EmissionSample]
}

func NewEmissionSampleRepository(db *gorm.DB) EmissionSampleRepository {
	return &emissionSampleRepository{store: NewStore[model.EmissionSample](db)}
}

func (r *emissionSampleRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.EmissionSample], error) {
	return r.store.List(ctx, q)
}
func (r *emissionSampleRepository) Get(ctx context.Context, id uint) (model.EmissionSample, error) {
	return r.store.Get(ctx, id)
}
func (r *emissionSampleRepository) Create(ctx context.Context, item *model.EmissionSample) error {
	return r.store.Create(ctx, item)
}
func (r *emissionSampleRepository) Update(ctx context.Context, id, version uint, item *model.EmissionSample) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *emissionSampleRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *emissionSampleRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}

// LatestVerifiedByRelatedCode returns the newest verified sample for the unit
// relation code on the caller's transaction, so the retire verification reads
// the same snapshot as its decision checks.
func (r *emissionSampleRepository) LatestVerifiedByRelatedCode(ctx context.Context, tx *gorm.DB, relatedCode string) (model.EmissionSample, error) {
	var sample model.EmissionSample
	err := tx.WithContext(ctx).
		Where("related_code = ? AND status = ?", strings.TrimSpace(relatedCode), "verified").
		Order("updated_at DESC, id DESC").
		First(&sample).Error
	return sample, err
}
