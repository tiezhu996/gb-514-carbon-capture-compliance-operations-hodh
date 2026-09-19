package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/constants"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/repository"
)

type PermitRuleService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.PermitRule], error)
	Get(context.Context, uint) (model.PermitRule, error)
	Create(context.Context, dto.CreatePermitRule, string, string) (model.PermitRule, error)
	Update(context.Context, uint, dto.UpdatePermitRule, string, string) (model.PermitRule, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.PermitRule, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type permitRuleService struct {
	repository repository.PermitRuleRepository
	security   SecurityService
}

func NewPermitRuleService(repo repository.PermitRuleRepository, security SecurityService) PermitRuleService {
	return &permitRuleService{repository: repo, security: security}
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
	if target == string(constants.PermitRuleStateRetired) {
		return model.PermitRule{}, ErrRetiredTransition
	}
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

func validatePermitRuleBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
