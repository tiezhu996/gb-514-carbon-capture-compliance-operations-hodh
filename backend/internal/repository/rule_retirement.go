package repository

import (
	"context"
	"errors"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InFlightDecisionStates are the compliance decision states that must be
// resolved before a device-scoped 许可规则 can be retired.
var InFlightDecisionStates = []string{"draft", "review"}

// RetirementQuerier is the read-only view used by 作废核验. It works both
// inside and outside transactions so the preflight API and the retire
// transaction share the exact same verification queries.
type RetirementQuerier interface {
	UnitByRelatedCode(ctx context.Context, relatedCode string, locked bool) (model.CaptureUnit, error)
	LatestVerifiedSample(ctx context.Context, relatedCode string, locked bool) (model.EmissionSample, error)
	InFlightDecisions(ctx context.Context, relatedCode string) ([]model.ComplianceDecision, error)
	ActivePermitRule(ctx context.Context, relatedCode string, locked bool) (model.PermitRule, error)
	PermitRuleByID(ctx context.Context, id uint, locked bool) (model.PermitRule, error)
}

// RetirementCoordinator serializes the device-scoped cross-aggregate work
// behind a single database transaction. Both rule retirement and new
// compliance decision submission take the same device key and row locks, so
// under concurrency only one side can commit; the other observes the changed
// state and fails without leaving a half-updated aggregate.
type RetirementCoordinator interface {
	RetirementQuerier
	// Run opens a transaction. When relatedCode is non-empty the permit row
	// for that device is locked first so concurrent retire/create requests
	// serialize on the same device key.
	Run(ctx context.Context, relatedCode string, fn func(tx DeviceTx) error) error
}

// DeviceTx is the transactional handle passed to service callbacks.
type DeviceTx interface {
	RetirementQuerier
	RetirePermitRule(ctx context.Context, id, expectedVersion uint, retired model.PermitRule) error
	CreateDecision(ctx context.Context, decision *model.ComplianceDecision, revision *model.DecisionRevision) error
	AppendAudit(ctx context.Context, audit *model.AuditLog) error
}

type retirementCoordinator struct {
	db *gorm.DB
}

func NewRetirementCoordinator(db *gorm.DB) RetirementCoordinator {
	return &retirementCoordinator{db: db}
}

func (c *retirementCoordinator) Run(ctx context.Context, relatedCode string, fn func(tx DeviceTx) error) error {
	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if relatedCode != "" && supportsRowLock(tx) {
			var locked model.PermitRule
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("related_code = ?", relatedCode).
				Order("id ASC").First(&locked).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		return fn(&deviceTx{tx: tx})
	})
}

func (c *retirementCoordinator) UnitByRelatedCode(ctx context.Context, relatedCode string, locked bool) (model.CaptureUnit, error) {
	return queryUnit(c.db.WithContext(ctx), relatedCode, locked)
}

func (c *retirementCoordinator) LatestVerifiedSample(ctx context.Context, relatedCode string, locked bool) (model.EmissionSample, error) {
	return queryLatestVerifiedSample(c.db.WithContext(ctx), relatedCode, locked)
}

func (c *retirementCoordinator) InFlightDecisions(ctx context.Context, relatedCode string) ([]model.ComplianceDecision, error) {
	return queryInFlightDecisions(c.db.WithContext(ctx), relatedCode)
}

func (c *retirementCoordinator) ActivePermitRule(ctx context.Context, relatedCode string, locked bool) (model.PermitRule, error) {
	return queryActivePermitRule(c.db.WithContext(ctx), relatedCode, locked)
}

func (c *retirementCoordinator) PermitRuleByID(ctx context.Context, id uint, locked bool) (model.PermitRule, error) {
	return queryPermitRuleByID(c.db.WithContext(ctx), id, locked)
}

type deviceTx struct {
	tx *gorm.DB
}

func (t *deviceTx) UnitByRelatedCode(ctx context.Context, relatedCode string, locked bool) (model.CaptureUnit, error) {
	return queryUnit(t.tx.WithContext(ctx), relatedCode, locked)
}

func (t *deviceTx) LatestVerifiedSample(ctx context.Context, relatedCode string, locked bool) (model.EmissionSample, error) {
	return queryLatestVerifiedSample(t.tx.WithContext(ctx), relatedCode, locked)
}

func (t *deviceTx) InFlightDecisions(ctx context.Context, relatedCode string) ([]model.ComplianceDecision, error) {
	return queryInFlightDecisions(t.tx.WithContext(ctx), relatedCode)
}

func (t *deviceTx) ActivePermitRule(ctx context.Context, relatedCode string, locked bool) (model.PermitRule, error) {
	return queryActivePermitRule(t.tx.WithContext(ctx), relatedCode, locked)
}

func (t *deviceTx) PermitRuleByID(ctx context.Context, id uint, locked bool) (model.PermitRule, error) {
	return queryPermitRuleByID(t.tx.WithContext(ctx), id, locked)
}

func (t *deviceTx) RetirePermitRule(ctx context.Context, id, expectedVersion uint, retired model.PermitRule) error {
	result := t.tx.WithContext(ctx).Model(&model.PermitRule{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Select("*").Omit("id", "code", "created_at", "deleted_at").Updates(&retired)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (t *deviceTx) CreateDecision(ctx context.Context, decision *model.ComplianceDecision, revision *model.DecisionRevision) error {
	if err := t.tx.WithContext(ctx).Omit("Revisions").Create(decision).Error; err != nil {
		return err
	}
	revision.ComplianceDecisionID = decision.ID
	return t.tx.WithContext(ctx).Create(revision).Error
}

func (t *deviceTx) AppendAudit(ctx context.Context, audit *model.AuditLog) error {
	return t.tx.WithContext(ctx).Create(audit).Error
}

func supportsRowLock(db *gorm.DB) bool {
	switch db.Dialector.Name() {
	case "postgres", "mysql":
		return true
	default:
		// SQLite (dev/test) has no SELECT ... FOR UPDATE; the service layer
		// adds an in-process device lock to preserve the same serialization.
		return false
	}
}

func lockingClause(db *gorm.DB, locked bool) *gorm.DB {
	if locked && supportsRowLock(db) {
		return db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return db
}

func queryUnit(db *gorm.DB, relatedCode string, locked bool) (model.CaptureUnit, error) {
	var item model.CaptureUnit
	err := lockingClause(db, locked).Where("related_code = ?", relatedCode).
		Order("updated_at DESC, id DESC").First(&item).Error
	return item, err
}

func queryLatestVerifiedSample(db *gorm.DB, relatedCode string, locked bool) (model.EmissionSample, error) {
	var item model.EmissionSample
	err := lockingClause(db, locked).Where("related_code = ? AND status = ?", relatedCode, "verified").
		Order("effective_at DESC, updated_at DESC, id DESC").First(&item).Error
	return item, err
}

func queryInFlightDecisions(db *gorm.DB, relatedCode string) ([]model.ComplianceDecision, error) {
	items := make([]model.ComplianceDecision, 0)
	err := db.Where("related_code = ? AND status IN ?", relatedCode, InFlightDecisionStates).
		Order("updated_at DESC, id DESC").Find(&items).Error
	return items, err
}

func queryActivePermitRule(db *gorm.DB, relatedCode string, locked bool) (model.PermitRule, error) {
	var item model.PermitRule
	err := lockingClause(db, locked).Where("related_code = ? AND status = ?", relatedCode, "active").
		Order("effective_at DESC, updated_at DESC, id DESC").First(&item).Error
	return item, err
}

func queryPermitRuleByID(db *gorm.DB, id uint, locked bool) (model.PermitRule, error) {
	var item model.PermitRule
	err := lockingClause(db, locked).First(&item, id).Error
	return item, err
}
