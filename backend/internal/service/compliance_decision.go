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

type ComplianceDecisionService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.ComplianceDecision], error)
	Get(context.Context, uint) (model.ComplianceDecision, error)
	Create(context.Context, dto.CreateComplianceDecision, string, string) (model.ComplianceDecision, error)
	Update(context.Context, uint, dto.UpdateComplianceDecision, string, string) (model.ComplianceDecision, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string, string) (model.ComplianceDecision, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type complianceDecisionService struct {
	repository  repository.ComplianceDecisionRepository
	coordinator repository.RetirementCoordinator
	security    SecurityService
	locker      DeviceLocker
}

func NewComplianceDecisionService(repo repository.ComplianceDecisionRepository, coordinator repository.RetirementCoordinator, security SecurityService, locker DeviceLocker) ComplianceDecisionService {
	return &complianceDecisionService{repository: repo, coordinator: coordinator, security: security, locker: locker}
}

func (s *complianceDecisionService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.ComplianceDecision], error) {
	return s.repository.List(ctx, query)
}

func (s *complianceDecisionService) Get(ctx context.Context, id uint) (model.ComplianceDecision, error) {
	return s.repository.Get(ctx, id)
}

func (s *complianceDecisionService) Create(ctx context.Context, input dto.CreateComplianceDecision, actor, requestID string) (model.ComplianceDecision, error) {
	if err := validateComplianceDecisionBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ComplianceDecision{}, err
	}
	item := model.ComplianceDecision{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.ComplianceDecisionInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	deviceKey := item.RelatedCode
	revision := newDecisionRevision(item.Version, item.Status, item.Evidence, "created compliance decision", actor, requestID, "", 0)
	if deviceKey == "" {
		// Legacy decisions without a device binding keep the original path and
		// carry an empty permit snapshot.
		if err := s.repository.CreateWithRevision(ctx, &item, revision); err != nil {
			return model.ComplianceDecision{}, fmt.Errorf("create 合规决定: %w", err)
		}
		_ = s.security.Audit(ctx, actor, requestID, "create", "ComplianceDecision", item.ID, "", item.Status, "created 合规决定")
		return s.repository.Get(ctx, item.ID)
	}

	// Device-scoped decisions resolve the active permit rule and snapshot its
	// code/version inside the same locked transaction as rule retirement, so a
	// concurrent retire and submit cannot both commit.
	err := s.locker.WithLock(ctx, deviceKey, func() error {
		return s.coordinator.Run(ctx, deviceKey, func(tx repository.DeviceTx) error {
			active, ruleErr := tx.ActivePermitRule(ctx, deviceKey, true)
			if ruleErr != nil {
				if errors.Is(ruleErr, gorm.ErrRecordNotFound) {
					return ErrNoActivePermitRule
				}
				return fmt.Errorf("resolve active 许可规则 for %s: %w", deviceKey, ruleErr)
			}
			item.PermitRuleCode = active.Code
			item.PermitRuleVersion = active.Version
			revision.PermitRuleCode = active.Code
			revision.PermitRuleVersion = active.Version
			if createErr := tx.CreateDecision(ctx, &item, revision); createErr != nil {
				return fmt.Errorf("create 合规决定: %w", createErr)
			}
			return tx.AppendAudit(ctx, &model.AuditLog{
				Actor: actor, RequestID: requestID, Action: "create", EntityType: "ComplianceDecision",
				EntityID: item.ID, BeforeState: "", AfterState: item.Status,
				Detail:    fmt.Sprintf("created 合规决定 referencing %s v%d", active.Code, active.Version),
				CreatedAt: time.Now().UTC(),
			})
		})
	})
	if err != nil {
		return model.ComplianceDecision{}, err
	}
	return s.repository.Get(ctx, item.ID)
}

func (s *complianceDecisionService) Update(ctx context.Context, id uint, input dto.UpdateComplianceDecision, actor, requestID string) (model.ComplianceDecision, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ComplianceDecision{}, err
	}
	if current.Status != string(constants.DecisionStateDraft) {
		return model.ComplianceDecision{}, ErrDecisionLocked
	}
	if err := validateComplianceDecisionBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ComplianceDecision{}, err
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
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(current.RelatedCode))
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	revision := newDecisionRevision(current.Version, current.Status, current.Evidence, "updated draft decision fields", actor, requestID, current.PermitRuleCode, current.PermitRuleVersion)
	if err := s.repository.UpdateWithRevision(ctx, id, input.ExpectedVersion, &current, revision); err != nil {
		return model.ComplianceDecision{}, fmt.Errorf("update 合规决定: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "ComplianceDecision", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *complianceDecisionService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, role, requestID string) (model.ComplianceDecision, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ComplianceDecision{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.ComplianceDecisionTransitions, current.Status, target) {
		return model.ComplianceDecision{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	if (target == string(constants.DecisionStateAccepted) || target == string(constants.DecisionStateEscalated)) &&
		role != model.RoleReviewer && role != model.RoleAdmin {
		return model.ComplianceDecision{}, ErrReviewerRequired
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	revision := newDecisionRevision(current.Version, target, current.Evidence, input.Reason, actor, requestID, current.PermitRuleCode, current.PermitRuleVersion)
	if err := s.repository.UpdateWithRevision(ctx, id, input.ExpectedVersion, &current, revision); err != nil {
		return model.ComplianceDecision{}, fmt.Errorf("transition 合规决定: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "ComplianceDecision", id, before, target, input.Reason); err != nil {
		return model.ComplianceDecision{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *complianceDecisionService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != model.ComplianceDecisionInitialStatus {
		return ErrDecisionLocked
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "ComplianceDecision", id, current.Status, "deleted", "soft deleted 合规决定")
}

func (s *complianceDecisionService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateComplianceDecisionBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}

func newDecisionRevision(version uint, state, evidence, reason, actor, requestID, permitRuleCode string, permitRuleVersion uint) *model.DecisionRevision {
	return &model.DecisionRevision{
		Version: version, State: strings.TrimSpace(state), Evidence: strings.TrimSpace(evidence),
		Reason: strings.TrimSpace(reason), Actor: strings.TrimSpace(actor),
		PermitRuleCode: strings.TrimSpace(permitRuleCode), PermitRuleVersion: permitRuleVersion,
		RequestID: strings.TrimSpace(requestID), CreatedAt: time.Now().UTC(),
	}
}
