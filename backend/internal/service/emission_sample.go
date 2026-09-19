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

type EmissionSampleService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.EmissionSample], error)
	Get(context.Context, uint) (model.EmissionSample, error)
	Create(context.Context, dto.CreateEmissionSample, string, string) (model.EmissionSample, error)
	Update(context.Context, uint, dto.UpdateEmissionSample, string, string) (model.EmissionSample, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.EmissionSample, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type emissionSampleService struct {
	repository repository.EmissionSampleRepository
	security   SecurityService
}

func NewEmissionSampleService(repo repository.EmissionSampleRepository, security SecurityService) EmissionSampleService {
	return &emissionSampleService{repository: repo, security: security}
}

func (s *emissionSampleService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.EmissionSample], error) {
	return s.repository.List(ctx, query)
}

func (s *emissionSampleService) Get(ctx context.Context, id uint) (model.EmissionSample, error) {
	return s.repository.Get(ctx, id)
}

func (s *emissionSampleService) Create(ctx context.Context, input dto.CreateEmissionSample, actor, requestID string) (model.EmissionSample, error) {
	if err := validateEmissionSampleBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.EmissionSample{}, err
	}
	item := model.EmissionSample{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.EmissionSampleInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.EmissionSample{}, fmt.Errorf("create 排放样本: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "EmissionSample", item.ID, "", item.Status, "created 排放样本")
	return item, nil
}

func (s *emissionSampleService) Update(ctx context.Context, id uint, input dto.UpdateEmissionSample, actor, requestID string) (model.EmissionSample, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.EmissionSample{}, err
	}
	if err := validateEmissionSampleBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.EmissionSample{}, err
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
		return model.EmissionSample{}, fmt.Errorf("update 排放样本: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "EmissionSample", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *emissionSampleService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.EmissionSample, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.EmissionSample{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.EmissionSampleTransitions, current.Status, target) {
		return model.EmissionSample{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.EmissionSample{}, fmt.Errorf("transition 排放样本: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "EmissionSample", id, before, target, input.Reason); err != nil {
		return model.EmissionSample{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *emissionSampleService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "EmissionSample", id, current.Status, "deleted", "soft deleted 排放样本")
}

func (s *emissionSampleService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateEmissionSampleBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
