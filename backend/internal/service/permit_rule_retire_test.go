package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/config"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/model"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type retireFixture struct {
	db        *gorm.DB
	rules     PermitRuleService
	decisions ComplianceDecisionService
}

func newRetireFixture(t *testing.T, dsn string) *retireFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.CaptureUnit{}, &model.PermitRule{}, &model.EmissionSample{},
		&model.ComplianceDecision{}, &model.DecisionRevision{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	decisionRepo := repository.NewComplianceDecisionRepository(db)
	return &retireFixture{
		db: db,
		rules: NewPermitRuleService(repository.NewPermitRuleRepository(db), decisionRepo,
			repository.NewEmissionSampleRepository(db), repository.NewCaptureUnitRepository(db), security),
		decisions: NewComplianceDecisionService(decisionRepo, security),
	}
}

func (f *retireFixture) seedUnit(t *testing.T, code, relatedCode, facility string) {
	t.Helper()
	unit := model.CaptureUnit{
		BaseModel: model.BaseModel{Code: code, Name: "捕集装置" + code, Status: "running", Version: 1},
		Facility:  facility, Owner: "运行组", Category: "常规", RiskLevel: "low",
		EffectiveAt: time.Now().UTC(), RelatedCode: relatedCode,
	}
	if err := f.db.Create(&unit).Error; err != nil {
		t.Fatalf("seed unit: %v", err)
	}
}

func (f *retireFixture) seedRule(t *testing.T, code, status, relatedCode, facility string) model.PermitRule {
	t.Helper()
	rule := model.PermitRule{
		BaseModel: model.BaseModel{Code: code, Name: "许可规则" + code, Status: status, Version: 1},
		Facility:  facility, Owner: "运行组", Category: "常规", RiskLevel: "medium",
		EffectiveAt: time.Now().UTC(), RelatedCode: relatedCode,
	}
	if err := f.db.Create(&rule).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	return rule
}

func (f *retireFixture) seedSample(t *testing.T, code, status, relatedCode, facility string) {
	t.Helper()
	sample := model.EmissionSample{
		BaseModel: model.BaseModel{Code: code, Name: "排放样本" + code, Status: status, Version: 1},
		Facility:  facility, Owner: "化验组", Category: "常规", RiskLevel: "low",
		EffectiveAt: time.Now().UTC(), RelatedCode: relatedCode,
	}
	if err := f.db.Create(&sample).Error; err != nil {
		t.Fatalf("seed sample: %v", err)
	}
}

func (f *retireFixture) seedDecision(t *testing.T, code, status, relatedCode string) {
	t.Helper()
	decision := model.ComplianceDecision{
		BaseModel: model.BaseModel{Code: code, Name: "合规决定" + code, Status: status, Version: 1},
		Facility:  "装置区一", Owner: "复核组", Category: "排放", RiskLevel: "high",
		EffectiveAt: time.Now().UTC(), RelatedCode: relatedCode,
	}
	if err := f.db.Omit("Revisions").Create(&decision).Error; err != nil {
		t.Fatalf("seed decision: %v", err)
	}
}

func blockerText(blockers []dto.RetireBlocker) string {
	parts := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		parts = append(parts, blocker.DecisionCode+"|"+blocker.Reason)
	}
	return strings.Join(parts, ";")
}

func TestPermitRuleRetireCheckListsInFlightDecisions(t *testing.T) {
	f := newRetireFixture(t, "file:retire-inflight?mode=memory&cache=shared")
	f.seedUnit(t, "CU-R1", "REL-R1", "装置区一")
	f.seedSample(t, "ES-R1", "verified", "REL-R1", "装置区一")
	rule := f.seedRule(t, "PR-R1", "active", "REL-R1", "装置区一")
	f.seedDecision(t, "CD-DRAFT", "draft", "REL-R1")
	f.seedDecision(t, "CD-REVIEW", "review", "PR-R1")
	f.seedDecision(t, "CD-ACCEPTED", "accepted", "REL-R1")

	check, err := f.rules.RetireCheck(context.Background(), rule.ID)
	if err != nil {
		t.Fatalf("retire check: %v", err)
	}
	if check.Allowed {
		t.Fatalf("in-flight decisions must block the retire: %+v", check)
	}
	text := blockerText(check.Blockers)
	if !strings.Contains(text, "CD-DRAFT") || !strings.Contains(text, "草稿") {
		t.Fatalf("draft decision blocker missing: %s", text)
	}
	if !strings.Contains(text, "CD-REVIEW") || !strings.Contains(text, "复核中") {
		t.Fatalf("review decision blocker missing: %s", text)
	}
	if strings.Contains(text, "CD-ACCEPTED") {
		t.Fatalf("accepted decisions must not block the retire: %s", text)
	}

	_, err = f.rules.Retire(context.Background(), rule.ID, dto.RetirePermitRule{
		ExpectedVersion: rule.Version, Reason: "尝试作废含在途决定的规则",
	}, "admin", model.RoleAdmin, "req-blocked")
	var blocked *RetireBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected RetireBlockedError, got %v", err)
	}
	if len(blocked.Blockers) != len(check.Blockers) {
		t.Fatalf("retire and check must report the same blockers: %+v vs %+v", blocked.Blockers, check.Blockers)
	}
	stored := f.seedRuleLookup(t, rule.ID)
	if stored.Status != "active" || stored.Version != rule.Version {
		t.Fatalf("blocked retire must not change the rule: %+v", stored)
	}
}

func (f *retireFixture) seedRuleLookup(t *testing.T, id uint) model.PermitRule {
	t.Helper()
	var rule model.PermitRule
	if err := f.db.First(&rule, id).Error; err != nil {
		t.Fatalf("reload rule: %v", err)
	}
	return rule
}

func TestPermitRuleRetireCheckSampleAndUnitBlockers(t *testing.T) {
	f := newRetireFixture(t, "file:retire-sample?mode=memory&cache=shared")
	f.seedUnit(t, "CU-R2", "REL-R2", "装置区二")
	missing := f.seedRule(t, "PR-R2", "active", "REL-R2", "装置区二")

	check, err := f.rules.RetireCheck(context.Background(), missing.ID)
	if err != nil {
		t.Fatalf("retire check: %v", err)
	}
	if check.Allowed || !strings.Contains(blockerText(check.Blockers), "最新已核验样本缺失") {
		t.Fatalf("missing verified sample must block: %+v", check)
	}

	f.seedSample(t, "ES-R2", "verified", "REL-R2", "装置区九")
	check, err = f.rules.RetireCheck(context.Background(), missing.ID)
	if err != nil {
		t.Fatalf("retire check: %v", err)
	}
	if check.Allowed || !strings.Contains(blockerText(check.Blockers), "样本装置不一致") {
		t.Fatalf("unit mismatch must block: %+v", check)
	}

	orphan := f.seedRule(t, "PR-R2B", "active", "REL-MISSING", "装置区二")
	check, err = f.rules.RetireCheck(context.Background(), orphan.ID)
	if err != nil {
		t.Fatalf("retire check: %v", err)
	}
	if check.Allowed || !strings.Contains(blockerText(check.Blockers), "关联捕集装置缺失") {
		t.Fatalf("missing unit must block: %+v", check)
	}
}

func TestPermitRuleRetireCheckSampleReadFailure(t *testing.T) {
	f := newRetireFixture(t, "file:retire-readfail?mode=memory&cache=shared")
	f.seedUnit(t, "CU-R3", "REL-R3", "装置区三")
	rule := f.seedRule(t, "PR-R3", "active", "REL-R3", "装置区三")
	if err := f.db.Migrator().DropTable(&model.EmissionSample{}); err != nil {
		t.Fatalf("drop sample table: %v", err)
	}
	check, err := f.rules.RetireCheck(context.Background(), rule.ID)
	if err != nil {
		t.Fatalf("sample read failure must surface as a blocker, not an error: %v", err)
	}
	if check.Allowed || !strings.Contains(blockerText(check.Blockers), "最新已核验样本读取失败") {
		t.Fatalf("sample read failure must block: %+v", check)
	}
}

func TestPermitRuleRetireRequiresAdmin(t *testing.T) {
	f := newRetireFixture(t, "file:retire-rbac?mode=memory&cache=shared")
	f.seedUnit(t, "CU-R4", "REL-R4", "装置区四")
	f.seedSample(t, "ES-R4", "verified", "REL-R4", "装置区四")
	rule := f.seedRule(t, "PR-R4", "active", "REL-R4", "装置区四")

	for _, role := range []string{model.RoleViewer, model.RoleOperator, model.RoleReviewer} {
		_, err := f.rules.Retire(context.Background(), rule.ID, dto.RetirePermitRule{
			ExpectedVersion: rule.Version, Reason: "非管理员尝试作废",
		}, "someone", role, "req-rbac")
		if !errors.Is(err, ErrAdminRequired) {
			t.Fatalf("role %s must be rejected with ErrAdminRequired, got %v", role, err)
		}
	}
	if stored := f.seedRuleLookup(t, rule.ID); stored.Status != "active" {
		t.Fatalf("rejected retire must not change the rule: %+v", stored)
	}
}

func TestPermitRuleRetireSuccessKeepsHistoryAndReferences(t *testing.T) {
	f := newRetireFixture(t, "file:retire-success?mode=memory&cache=shared")
	ctx := context.Background()
	f.seedUnit(t, "CU-R5", "REL-R5", "装置区五")
	f.seedSample(t, "ES-R5", "verified", "REL-R5", "装置区五")
	rule := f.seedRule(t, "PR-R5", "active", "REL-R5", "装置区五")
	f.seedDecision(t, "CD-KEEP", "accepted", "PR-R5")

	check, err := f.rules.RetireCheck(ctx, rule.ID)
	if err != nil {
		t.Fatalf("retire check: %v", err)
	}
	if !check.Allowed || len(check.Blockers) != 0 {
		t.Fatalf("clean unit must allow retire: %+v", check)
	}

	retired, err := f.rules.Retire(ctx, rule.ID, dto.RetirePermitRule{
		ExpectedVersion: rule.Version, Reason: "装置停用, 作废许可",
	}, "admin", model.RoleAdmin, "req-retire-ok")
	if err != nil {
		t.Fatalf("retire: %v", err)
	}
	if retired.Status != "retired" || retired.Version != rule.Version+1 {
		t.Fatalf("retire must bump version and keep the row: %+v", retired)
	}

	var kept model.ComplianceDecision
	if err := f.db.Where("code = ?", "CD-KEEP").First(&kept).Error; err != nil {
		t.Fatalf("accepted decision must survive the retire: %v", err)
	}
	if kept.Status != "accepted" || kept.RelatedCode != "PR-R5" {
		t.Fatalf("accepted decision must keep referencing the original rule: %+v", kept)
	}
	if _, err := f.rules.Get(ctx, rule.ID); err != nil {
		t.Fatalf("retired rule must stay readable for history: %v", err)
	}

	_, err = f.decisions.Create(ctx, dto.CreateComplianceDecision{
		Code: "CD-LATE", Name: "作废后提交的决定", Facility: "装置区五", Owner: "operator",
		Category: "排放", RiskLevel: "high", EffectiveAt: time.Now().UTC(), RelatedCode: "PR-R5",
	}, "operator", "req-after-retire")
	if !errors.Is(err, repository.ErrRuleRetired) {
		t.Fatalf("new decisions must not reference a retired rule, got %v", err)
	}

	check, err = f.rules.RetireCheck(ctx, rule.ID)
	if err != nil {
		t.Fatalf("retire check on retired rule: %v", err)
	}
	if check.Allowed || !strings.Contains(blockerText(check.Blockers), "不允许作废") {
		t.Fatalf("retired rule must not be retireable again: %+v", check)
	}
}

func TestPermitRuleRetireAndDecisionCreateAreMutuallyExclusive(t *testing.T) {
	f := newRetireFixture(t, "file:retire-race?mode=memory&cache=shared")
	ctx := context.Background()
	f.seedUnit(t, "CU-R6", "REL-R6", "装置区六")
	f.seedSample(t, "ES-R6", "verified", "REL-R6", "装置区六")
	rule := f.seedRule(t, "PR-R6", "active", "REL-R6", "装置区六")

	start := make(chan struct{})
	var wg sync.WaitGroup
	var retireErr, createErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, retireErr = f.rules.Retire(ctx, rule.ID, dto.RetirePermitRule{
			ExpectedVersion: rule.Version, Reason: "并发作废核验",
		}, "admin", model.RoleAdmin, "req-race-retire")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, createErr = f.decisions.Create(ctx, dto.CreateComplianceDecision{
			Code: "CD-RACE", Name: "并发提交的决定", Facility: "装置区六", Owner: "operator",
			Category: "排放", RiskLevel: "high", EffectiveAt: time.Now().UTC(), RelatedCode: "PR-R6",
		}, "operator", "req-race-create")
	}()
	close(start)
	wg.Wait()

	stored := f.seedRuleLookup(t, rule.ID)
	var decisionCount int64
	if err := f.db.Model(&model.ComplianceDecision{}).Where("code = ?", "CD-RACE").Count(&decisionCount).Error; err != nil {
		t.Fatalf("count decisions: %v", err)
	}
	retired, created := stored.Status == "retired", decisionCount == 1
	if retired == created {
		t.Fatalf("exactly one side may commit: retired=%v created=%v retireErr=%v createErr=%v",
			retired, created, retireErr, createErr)
	}
	if retired {
		if createErr == nil {
			t.Fatalf("decision create must fail once the rule is retired")
		}
		if stored.Version != rule.Version+1 {
			t.Fatalf("retire must bump the version exactly once: %+v", stored)
		}
		return
	}
	if retireErr == nil {
		t.Fatalf("retire must fail once the decision is in flight")
	}
	if createErr != nil {
		t.Fatalf("winning decision create must not report an error: %v", createErr)
	}
	if stored.Status != "active" || stored.Version != rule.Version {
		t.Fatalf("failed retire must not leave a half update: %+v", stored)
	}
}
