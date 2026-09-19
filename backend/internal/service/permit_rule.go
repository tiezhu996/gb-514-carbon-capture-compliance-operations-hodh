package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/constants"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/repository"
	"gorm.io/gorm"
)

type PermitRuleService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.PermitRule], error)
	Get(context.Context, uint) (model.PermitRule, error)
	Create(context.Context, dto.CreatePermitRule, string, string) (model.PermitRule, error)
	Update(context.Context, uint, dto.UpdatePermitRule, string, string) (model.PermitRule, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.PermitRule, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
	RetireCheck(context.Context, uint) (dto.RuleRetireCheck, error)
	Retire(context.Context, uint, dto.RetirePermitRule, string, string, string) (model.PermitRule, error)
}

type permitRuleService struct {
	repository repository.PermitRuleRepository
	decisions  repository.ComplianceDecisionRepository
	samples    repository.EmissionSampleRepository
	units      repository.CaptureUnitRepository
	security   SecurityService
}

func NewPermitRuleService(repo repository.PermitRuleRepository, decisions repository.ComplianceDecisionRepository, samples repository.EmissionSampleRepository, units repository.CaptureUnitRepository, security SecurityService) PermitRuleService {
	return &permitRuleService{repository: repo, decisions: decisions, samples: samples, units: units, security: security}
}

func (s *permitRuleService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.PermitRule], error) {
	return s.repository.List(ctx, query)
}

func (s *permitRuleService) Get(ctx context.Context, id uint) (model.PermitRule, error) {
	return s.repository.Get(ctx, id)
}

func (s *permitRuleService) Create(ctx context.Context, input dto.CreatePermitRule, actor, requestID string) (model.PermitRule, error) {
	if err := validatePermitRuleBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.PermitRule{}, err
	}
	item := model.PermitRule{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.PermitRuleInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.PermitRule{}, fmt.Errorf("create 许可规则: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "PermitRule", item.ID, "", item.Status, "created 许可规则")
	return item, nil
}

func (s *permitRuleService) Update(ctx context.Context, id uint, input dto.UpdatePermitRule, actor, requestID string) (model.PermitRule, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.PermitRule{}, err
	}
	if err := validatePermitRuleBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.PermitRule{}, err
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = strings.TrimSpace(input.Facility)
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.PermitRule{}, fmt.Errorf("update 许可规则: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "PermitRule", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *permitRuleService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.PermitRule, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.PermitRule{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.PermitRuleTransitions, current.Status, target) {
		return model.PermitRule{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.PermitRule{}, fmt.Errorf("transition 许可规则: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "PermitRule", id, before, target, input.Reason); err != nil {
		return model.PermitRule{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *permitRuleService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "PermitRule", id, current.Status, "deleted", "soft deleted 许可规则")
}

func (s *permitRuleService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

// RetireCheck runs the per-unit verification without writing anything so the
// rule page can list blocking decision codes and reasons up front.
func (s *permitRuleService) RetireCheck(ctx context.Context, id uint) (dto.RuleRetireCheck, error) {
	blockers := make([]dto.RetireBlocker, 0)
	rule, err := s.repository.InspectForRetire(ctx, id, func(tx *gorm.DB, locked model.PermitRule) error {
		found, verifyErr := s.collectRetireBlockers(ctx, tx, locked)
		if verifyErr != nil {
			return verifyErr
		}
		blockers = found
		return nil
	})
	if err != nil {
		return dto.RuleRetireCheck{}, err
	}
	return dto.RuleRetireCheck{
		RuleID: rule.ID, RuleCode: rule.Code, Allowed: len(blockers) == 0,
		Blockers: blockers, CheckedAt: time.Now().UTC(),
	}, nil
}

// Retire voids a permit rule. Only admins may retire, the per-unit
// verification must pass inside the same transaction that flips the status,
// and the rule row is kept (status retired) so history and accepted decisions
// keep referencing the original version.
func (s *permitRuleService) Retire(ctx context.Context, id uint, input dto.RetirePermitRule, actor, role, requestID string) (model.PermitRule, error) {
	if role != model.RoleAdmin {
		return model.PermitRule{}, ErrAdminRequired
	}
	before := ""
	retired, err := s.repository.Retire(ctx, id, input.ExpectedVersion, func(tx *gorm.DB, locked model.PermitRule) error {
		if !constants.CanTransition(constants.PermitRuleTransitions, locked.Status, "retired") {
			return fmt.Errorf("%w: %s -> retired", ErrInvalidTransition, locked.Status)
		}
		before = locked.Status
		blockers, verifyErr := s.collectRetireBlockers(ctx, tx, locked)
		if verifyErr != nil {
			return verifyErr
		}
		if len(blockers) > 0 {
			return &RetireBlockedError{Blockers: blockers}
		}
		return nil
	})
	if err != nil {
		return model.PermitRule{}, err
	}
	if err := s.security.Audit(ctx, actor, requestID, "retire", "PermitRule", id, before, "retired", input.Reason); err != nil {
		return model.PermitRule{}, fmt.Errorf("persist retire audit: %w", err)
	}
	return retired, nil
}

// collectRetireBlockers verifies, scoped to the rule's capture unit, the
// in-flight compliance decisions and the latest verified sample. Missing
// samples, unit mismatches and read failures all become blockers; the caller
// refuses the retire whenever the list is non-empty.
func (s *permitRuleService) collectRetireBlockers(ctx context.Context, tx *gorm.DB, rule model.PermitRule) ([]dto.RetireBlocker, error) {
	blockers := make([]dto.RetireBlocker, 0)
	if !constants.CanTransition(constants.PermitRuleTransitions, rule.Status, "retired") {
		blockers = append(blockers, dto.RetireBlocker{Reason: fmt.Sprintf("当前状态 %s 不允许作废", rule.Status)})
	}
	inFlight, err := s.decisions.InFlightByRuleRef(ctx, tx, rule.Code, rule.RelatedCode)
	if err != nil {
		return nil, fmt.Errorf("read in-flight 合规决定: %w", err)
	}
	for _, decision := range inFlight {
		blockers = append(blockers, dto.RetireBlocker{
			DecisionCode: decision.Code,
			Reason:       fmt.Sprintf("存在%s的关联决定", inFlightStateLabel(decision.Status)),
		})
	}
	unit, unitErr := s.units.GetByRelatedCode(ctx, tx, rule.RelatedCode)
	switch {
	case errors.Is(unitErr, gorm.ErrRecordNotFound):
		blockers = append(blockers, dto.RetireBlocker{Reason: "关联捕集装置缺失"})
	case unitErr != nil:
		blockers = append(blockers, dto.RetireBlocker{Reason: "捕集装置读取失败"})
	}
	sample, sampleErr := s.samples.LatestVerifiedByRelatedCode(ctx, tx, rule.RelatedCode)
	switch {
	case errors.Is(sampleErr, gorm.ErrRecordNotFound):
		blockers = append(blockers, dto.RetireBlocker{Reason: "最新已核验样本缺失"})
	case sampleErr != nil:
		blockers = append(blockers, dto.RetireBlocker{Reason: "最新已核验样本读取失败"})
	case unit.ID != 0 && sample.Facility != unit.Facility:
		blockers = append(blockers, dto.RetireBlocker{
			Reason: fmt.Sprintf("样本装置不一致: 样本 %s 属于 %s, 装置属于 %s", sample.Code, sample.Facility, unit.Facility),
		})
	}
	return blockers, nil
}

func inFlightStateLabel(status string) string {
	if status == string(constants.DecisionStateReview) {
		return "复核中"
	}
	return "草稿"
}

func validatePermitRuleBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
