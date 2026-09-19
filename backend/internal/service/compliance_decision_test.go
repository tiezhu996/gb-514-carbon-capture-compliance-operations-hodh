package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/config"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestComplianceDecisionPreservesEveryVersionAndReviewerBoundary(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:decision-versioning?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.ComplianceDecision{}, &model.DecisionRevision{}, &model.AuditLog{},
		&model.PermitRule{}, &model.CaptureUnit{}, &model.EmissionSample{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	decisionRepo := repository.NewComplianceDecisionRepository(db)
	coordinator := repository.NewRetirementCoordinator(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	svc := NewComplianceDecisionService(decisionRepo, coordinator, security, NewDeviceLocker())
	ruleSvc := NewPermitRuleService(repository.NewPermitRuleRepository(db), security)
	ctx := context.Background()
	now := time.Now().UTC()

	activeRule, err := ruleSvc.Create(ctx, dto.CreatePermitRule{
		Code: "PR-TEST", Name: "Stack permit rule", Description: "active threshold for test train",
		Facility: "Capture train A", Owner: "operator", Category: "emissions", RiskLevel: "high",
		MetricValue: 40, MetricUnit: "ppm", EffectiveAt: now,
		Evidence: "permit issued", RelatedCode: "PR-TEST",
	}, "admin", "request-rule-create")
	if err != nil {
		t.Fatalf("create draft rule: %v", err)
	}
	activeRule, err = ruleSvc.Transition(ctx, activeRule.ID, dto.TransitionRequest{
		Status: "active", ExpectedVersion: activeRule.Version, Reason: "permit enters force for decision tests",
	}, "admin", "request-rule-active")
	if err != nil {
		t.Fatalf("activate rule: %v", err)
	}

	created, err := svc.Create(ctx, dto.CreateComplianceDecision{
		Code: "CD-TEST", Name: "Stack compliance decision", Description: "initial assessment",
		Facility: "Capture train A", Owner: "operator", Category: "emissions", RiskLevel: "high",
		MetricValue: 31.5, MetricUnit: "ppm", EffectiveAt: now,
		Evidence: "sample ES-TEST and permit PR-TEST", RelatedCode: "PR-TEST",
	}, "operator", "request-create")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	if created.Version != 1 || len(created.Revisions) != 1 || created.Revisions[0].RequestID != "request-create" {
		t.Fatalf("initial revision context missing: %+v", created.Revisions)
	}
	if created.PermitRuleCode != "PR-TEST" || created.PermitRuleVersion != activeRule.Version {
		t.Fatalf("decision should snapshot active permit rule, got code=%q version=%d", created.PermitRuleCode, created.PermitRuleVersion)
	}
	if created.Revisions[0].PermitRuleCode != "PR-TEST" || created.Revisions[0].PermitRuleVersion != activeRule.Version {
		t.Fatalf("revision snapshot missing: %+v", created.Revisions[0])
	}

	updated, err := svc.Update(ctx, created.ID, dto.UpdateComplianceDecision{
		ExpectedVersion: 1, Name: "Stack compliance decision", Description: "expanded assessment",
		Facility: "Capture train A", Owner: "operator", Category: "emissions", RiskLevel: "critical",
		MetricValue: 35, MetricUnit: "ppm", EffectiveAt: now,
		Evidence: "sample ES-TEST, permit PR-TEST, calibrated analyzer", RelatedCode: "PR-TEST",
	}, "operator", "request-update")
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if updated.Version != 2 || len(updated.Revisions) != 2 || updated.Revisions[1].Actor != "operator" ||
		updated.Revisions[0].Evidence == updated.Revisions[1].Evidence {
		t.Fatalf("draft revisions were overwritten or incomplete: %+v", updated.Revisions)
	}

	reviewed, err := svc.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: 2, Reason: "evidence package is ready for independent review",
	}, "operator", model.RoleOperator, "request-review")
	if err != nil {
		t.Fatalf("submit review: %v", err)
	}
	if reviewed.Status != "review" || len(reviewed.Revisions) != 3 || reviewed.Revisions[2].State != "review" {
		t.Fatalf("review revision missing: %+v", reviewed)
	}

	_, err = svc.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "accepted", ExpectedVersion: 3, Reason: "operator attempted final acceptance",
	}, "operator", model.RoleOperator, "request-denied")
	if !errors.Is(err, ErrReviewerRequired) {
		t.Fatalf("expected reviewer boundary, got %v", err)
	}

	accepted, err := svc.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "accepted", ExpectedVersion: 3, Reason: "permit threshold and calibrated evidence agree",
	}, "reviewer", model.RoleReviewer, "request-accepted")
	if err != nil {
		t.Fatalf("accept decision: %v", err)
	}
	if accepted.Status != "accepted" || len(accepted.Revisions) != 4 ||
		accepted.Revisions[3].Actor != "reviewer" || accepted.Revisions[3].RequestID != "request-accepted" ||
		accepted.Revisions[0].RequestID != "request-create" {
		t.Fatalf("immutable decision history is incomplete: %+v", accepted.Revisions)
	}
	if accepted.PermitRuleCode != "PR-TEST" || accepted.PermitRuleVersion != activeRule.Version ||
		accepted.Revisions[3].PermitRuleCode != "PR-TEST" {
		t.Fatalf("accepted decision must keep referencing the original permit version: %+v", accepted)
	}

	_, err = svc.Update(ctx, created.ID, dto.UpdateComplianceDecision{ExpectedVersion: accepted.Version}, "admin", "request-late-update")
	if !errors.Is(err, ErrDecisionLocked) {
		t.Fatalf("expected reviewed decision to be locked, got %v", err)
	}
}
