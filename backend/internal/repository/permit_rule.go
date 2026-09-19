package repository

import (
	"context"
	"time"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"gorm.io/gorm"
)

// PermitRuleRepository owns all persistence operations for 许可规则.
type PermitRuleRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.PermitRule], error)
	Get(context.Context, uint) (model.PermitRule, error)
	Create(context.Context, *model.PermitRule) error
	Update(context.Context, uint, uint, *model.PermitRule) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
	InspectForRetire(context.Context, uint, RetireVerification) (model.PermitRule, error)
	Retire(context.Context, uint, uint, RetireVerification) (model.PermitRule, error)
}

// RetireVerification runs inside the retire transaction while the rule row is
// locked, so its reads and the status update commit or roll back together.
type RetireVerification func(tx *gorm.DB, rule model.PermitRule) error

type permitRuleRepository struct {
	store *Store[model.PermitRule]
	db    *gorm.DB
}

func NewPermitRuleRepository(db *gorm.DB) PermitRuleRepository {
	return &permitRuleRepository{store: NewStore[model.PermitRule](db), db: db}
}

func (r *permitRuleRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.PermitRule], error) {
	return r.store.List(ctx, q)
}
func (r *permitRuleRepository) Get(ctx context.Context, id uint) (model.PermitRule, error) {
	return r.store.Get(ctx, id)
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

// InspectForRetire loads the rule under the same row lock the retire uses and
// runs the verification callback without persisting anything, giving the rule
// page a read-only preview of the blockers.
func (r *permitRuleRepository) InspectForRetire(ctx context.Context, id uint, verify RetireVerification) (model.PermitRule, error) {
	var locked model.PermitRule
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockRow(tx).WithContext(ctx).First(&locked, id).Error; err != nil {
			return err
		}
		if verify == nil {
			return nil
		}
		return verify(tx, locked)
	})
	return locked, err
}

// Retire verifies and retires the rule in one transaction: the rule row is
// locked, the verification runs against the same snapshot, and the optimistic
// version update commits only when verification passed. Any error rolls the
// whole transaction back so a failed retire never leaves a half update.
func (r *permitRuleRepository) Retire(ctx context.Context, id, expectedVersion uint, verify RetireVerification) (model.PermitRule, error) {
	var locked model.PermitRule
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockRow(tx).WithContext(ctx).First(&locked, id).Error; err != nil {
			return err
		}
		if verify != nil {
			if err := verify(tx, locked); err != nil {
				return err
			}
		}
		result := tx.Model(&model.PermitRule{}).
			Where("id = ? AND version = ?", id, expectedVersion).
			Updates(map[string]any{
				"status":     "retired",
				"version":    expectedVersion + 1,
				"updated_at": time.Now().UTC(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrVersionConflict
		}
		locked.Status = "retired"
		locked.Version = expectedVersion + 1
		locked.UpdatedAt = time.Now().UTC()
		return nil
	})
	return locked, err
}
