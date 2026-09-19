package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/constants"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/repository"
	"gorm.io/gorm"
)

// Retirement block reason codes. They are stable strings used by the rule page
// to render why each 许可规则 cannot be retired.
const (
	RetirementBlockRuleState      = "rule_state"
	RetirementBlockDeviceMissing  = "device_missing"
	RetirementBlockDeviceRead     = "device_read_failed"
	RetirementBlockSampleMissing  = "sample_missing"
	RetirementBlockSampleRead     = "sample_read_failed"
	RetirementBlockDeviceMismatch = "device_mismatch"
	RetirementBlockInFlight       = "in_flight_decision"
)

// RetirementBlocker describes a single reason that prevents retiring a rule.
// Code carries the machine-readable category, DecisionCode is populated for
// in-flight 合规决定 blocks and Reason explains it in Chinese for the rule page.
type RetirementBlocker struct {
	Code         string `json:"code"`
	DecisionCode string `json:"decisionCode,omitempty"`
	Reason       string `json:"reason"`
}

// RetirementCheck is the preflight result returned to the rule page.
type RetirementCheck struct {
	PermitRuleCode string              `json:"permitRuleCode"`
	RelatedCode    string              `json:"relatedCode"`
	Allowed        bool                `json:"allowed"`
	DeviceCode     string              `json:"deviceCode,omitempty"`
	SampleCode     string              `json:"sampleCode,omitempty"`
	Blockers       []RetirementBlocker `json:"blockers"`
}

// RetirementBlockError carries the full blocker list so the handler can return
// 422 with each 决定编号 and its blocking reason.
type RetirementBlockError struct {
	Blockers []RetirementBlocker
}

func (e *RetirementBlockError) Error() string {
	return fmt.Sprintf("%s: %d blocker(s)", ErrRetirementBlocked.Error(), len(e.Blockers))
}
func (e *RetirementBlockError) Unwrap() error { return ErrRetirementBlocked }

type RuleRetirementService interface {
	// Precheck runs the device-scoped verification read-only and returns every
	// blocker (or an Allowed check). It never mutates the rule.
	Precheck(ctx context.Context, ruleID uint) (RetirementCheck, error)
	// Retire re-runs the verification inside the device-locked transaction and,
	// only when no blocker exists, moves the rule to retired. Only admin may
	// call it; failures roll the whole transaction back.
	Retire(ctx context.Context, ruleID uint, expectedVersion uint, role, actor, requestID, reason string) (model.PermitRule, error)
}

type ruleRetirementService struct {
	rules       repository.PermitRuleRepository
	coordinator repository.RetirementCoordinator
	security    SecurityService
	locker      DeviceLocker
}

func NewRuleRetirementService(rules repository.PermitRuleRepository, coordinator repository.RetirementCoordinator, security SecurityService, locker DeviceLocker) RuleRetirementService {
	return &ruleRetirementService{rules: rules, coordinator: coordinator, security: security, locker: locker}
}

func (s *ruleRetirementService) Precheck(ctx context.Context, ruleID uint) (RetirementCheck, error) {
	rule, err := s.rules.Get(ctx, ruleID)
	if err != nil {
		return RetirementCheck{}, err
	}
	check := RetirementCheck{PermitRuleCode: rule.Code, RelatedCode: rule.RelatedCode, Blockers: make([]RetirementBlocker, 0)}
	err = s.coordinator.Run(ctx, "", func(tx repository.DeviceTx) error {
		return s.verify(ctx, tx, rule, &check)
	})
	if err != nil {
		return RetirementCheck{}, err
	}
	check.Allowed = len(check.Blockers) == 0
	return check, nil
}

func (s *ruleRetirementService) Retire(ctx context.Context, ruleID uint, expectedVersion uint, role, actor, requestID, reason string) (model.PermitRule, error) {
	if role != model.RoleAdmin {
		return model.PermitRule{}, ErrAdminRequired
	}
	rule, err := s.rules.Get(ctx, ruleID)
	if err != nil {
		return model.PermitRule{}, err
	}
	deviceKey := strings.ToUpper(strings.TrimSpace(rule.RelatedCode))
	err = s.locker.WithLock(ctx, deviceKey, func() error {
		return s.coordinator.Run(ctx, deviceKey, func(tx repository.DeviceTx) error {
			locked, lockErr := tx.PermitRuleByID(ctx, ruleID, true)
			if lockErr != nil {
				return lockErr
			}
			check := RetirementCheck{PermitRuleCode: locked.Code, RelatedCode: locked.RelatedCode, Blockers: make([]RetirementBlocker, 0)}
			if verifyErr := s.verify(ctx, tx, locked, &check); verifyErr != nil {
				return verifyErr
			}
			if len(check.Blockers) > 0 {
				return &RetirementBlockError{Blockers: check.Blockers}
			}
			before := locked.Status
			locked.Status = string(constants.PermitRuleStateRetired)
			locked.Version = expectedVersion + 1
			locked.UpdatedAt = time.Now().UTC()
			if updateErr := tx.RetirePermitRule(ctx, ruleID, expectedVersion, locked); updateErr != nil {
				return updateErr
			}
			audit := &model.AuditLog{
				Actor: actor, RequestID: requestID, Action: "retire", EntityType: "PermitRule",
				EntityID: ruleID, BeforeState: before, AfterState: locked.Status,
				Detail: strings.TrimSpace(reason), CreatedAt: time.Now().UTC(),
			}
			return tx.AppendAudit(ctx, audit)
		})
	})
	if err != nil {
		return model.PermitRule{}, err
	}
	return s.rules.Get(ctx, ruleID)
}

// verify performs the actual 按装置核验. It appends blockers rather than
// returning on the first business failure so the rule page can list every
// blocking 决定编号 and reason at once. Missing records and facility mismatches
// are business blockers; genuine read failures (database/IO errors) return an
// error so retirement is refused outright and no half-update is possible.
func (s *ruleRetirementService) verify(ctx context.Context, tx repository.RetirementQuerier, rule model.PermitRule, check *RetirementCheck) error {
	if rule.Status == string(constants.PermitRuleStateRetired) {
		check.Blockers = append(check.Blockers, RetirementBlocker{
			Code: RetirementBlockRuleState, Reason: fmt.Sprintf("许可规则 %s 已作废，不能重复作废", rule.Code),
		})
		return nil
	}
	if !constants.CanTransition(constants.PermitRuleTransitions, rule.Status, string(constants.PermitRuleStateRetired)) {
		check.Blockers = append(check.Blockers, RetirementBlocker{
			Code: RetirementBlockRuleState, Reason: fmt.Sprintf("许可规则 %s 当前状态为 %s，不允许迁移到 retired", rule.Code, rule.Status),
		})
	}
	deviceKey := strings.ToUpper(strings.TrimSpace(rule.RelatedCode))
	if deviceKey == "" {
		check.Blockers = append(check.Blockers, RetirementBlocker{
			Code: RetirementBlockDeviceMissing, Reason: fmt.Sprintf("许可规则 %s 未绑定捕集装置（relatedCode 为空）", rule.Code),
		})
		return nil
	}

	unit, unitErr := tx.UnitByRelatedCode(ctx, deviceKey, false)
	if unitErr != nil {
		if !errors.Is(unitErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("核验装置 %s 读取失败: %w", deviceKey, unitErr)
		}
		check.Blockers = append(check.Blockers, RetirementBlocker{
			Code:   RetirementBlockDeviceMissing,
			Reason: fmt.Sprintf("未找到装置编码 %s 对应的捕集装置", deviceKey),
		})
	} else {
		check.DeviceCode = unit.Code
		if strings.TrimSpace(unit.Facility) != strings.TrimSpace(rule.Facility) {
			check.Blockers = append(check.Blockers, RetirementBlocker{
				Code:   RetirementBlockDeviceMismatch,
				Reason: fmt.Sprintf("装置 %s 所属设施 %q 与许可规则设施 %q 不一致", unit.Code, unit.Facility, rule.Facility),
			})
		}
	}

	sample, sampleErr := tx.LatestVerifiedSample(ctx, deviceKey, false)
	if sampleErr != nil {
		if !errors.Is(sampleErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("核验装置 %s 已核验样本读取失败: %w", deviceKey, sampleErr)
		}
		check.Blockers = append(check.Blockers, RetirementBlocker{
			Code:   RetirementBlockSampleMissing,
			Reason: fmt.Sprintf("装置 %s 缺少最新已核验（verified）排放样本", deviceKey),
		})
	} else {
		check.SampleCode = sample.Code
		if strings.TrimSpace(sample.Facility) != strings.TrimSpace(rule.Facility) {
			check.Blockers = append(check.Blockers, RetirementBlocker{
				Code:   RetirementBlockDeviceMismatch,
				Reason: fmt.Sprintf("样本 %s 所属设施 %q 与许可规则设施 %q 不一致", sample.Code, sample.Facility, rule.Facility),
			})
		}
	}

	decisions, decisionErr := tx.InFlightDecisions(ctx, deviceKey)
	if decisionErr != nil {
		return fmt.Errorf("核验装置 %s 在途合规决定读取失败: %w", deviceKey, decisionErr)
	}
	for _, decision := range decisions {
		stateLabel := map[string]string{"draft": "草稿", "review": "复核中"}[decision.Status]
		check.Blockers = append(check.Blockers, RetirementBlocker{
			Code: RetirementBlockInFlight, DecisionCode: decision.Code,
			Reason: fmt.Sprintf("关联合规决定 %s 处于%s状态，必须先完成或退回后才能作废许可规则", decision.Code, stateLabel),
		})
	}
	return nil
}
