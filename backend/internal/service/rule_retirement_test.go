package service

import (
	"context"
	"errors"
	"fmt"
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

type retirementFixture struct {
	db         *gorm.DB
	locker     DeviceLocker
	rules      repository.PermitRuleRepository
	units      repository.CaptureUnitRepository
	samples    repository.EmissionSampleRepository
	decisions  repository.ComplianceDecisionRepository
	retirement RuleRetirementService
	decision   ComplianceDecisionService
	rule       PermitRuleService
}

func newRetirementFixture(t *testing.T, dsn string) retirementFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.AuditLog{}, &model.CaptureUnit{}, &model.PermitRule{},
		&model.EmissionSample{}, &model.ComplianceDecision{}, &model.DecisionRevision{},
	); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	coordinator := repository.NewRetirementCoordinator(db)
	locker := NewDeviceLocker()
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	ruleRepo := repository.NewPermitRuleRepository(db)
	unitRepo := repository.NewCaptureUnitRepository(db)
	sampleRepo := repository.NewEmissionSampleRepository(db)
	decisionRepo := repository.NewComplianceDecisionRepository(db)
	return retirementFixture{
		db: db, locker: locker, rules: ruleRepo, units: unitRepo, samples: sampleRepo,
		decisions:  decisionRepo,
		retirement: NewRuleRetirementService(ruleRepo, coordinator, security, locker),
		decision:   NewComplianceDecisionService(decisionRepo, coordinator, security, locker),
		rule:       NewPermitRuleService(ruleRepo, security),
	}
}

func seedRetirementWorld(t *testing.T, f retirementFixture, device string, decisionStates []string) model.PermitRule {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	unit := model.CaptureUnit{
		BaseModel: model.BaseModel{Code: "CU-" + device, Name: "装置 " + device, Status: "running", Version: 1},
		Facility:  "厂区A", Owner: "运行组", Category: "常规", RiskLevel: "low",
		MetricUnit: "ppm", EffectiveAt: now, RelatedCode: device,
	}
	if err := f.units.Create(ctx, &unit); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	sample := model.EmissionSample{
		BaseModel: model.BaseModel{Code: "ES-" + device, Name: "样本 " + device, Status: "verified", Version: 1},
		Facility:  "厂区A", Owner: "运行组", Category: "常规", RiskLevel: "low",
		MetricUnit: "ppm", EffectiveAt: now, RelatedCode: device,
	}
	if err := f.samples.Create(ctx, &sample); err != nil {
		t.Fatalf("seed sample: %v", err)
	}
	rule := model.PermitRule{
		BaseModel: model.BaseModel{Code: "PR-" + device, Name: "规则 " + device, Status: "active", Version: 3},
		Facility:  "厂区A", Owner: "运行组", Category: "常规", RiskLevel: "medium",
		MetricValue: 30, MetricUnit: "ppm", EffectiveAt: now, RelatedCode: device,
	}
	if err := f.rules.Create(ctx, &rule); err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	for index, state := range decisionStates {
		decision := model.ComplianceDecision{
			BaseModel: model.BaseModel{
				Code: fmt.Sprintf("CD-%s-%d", device, index+1), Name: "决定 " + device,
				Status: state, Version: 1,
			},
			Facility: "厂区A", Owner: "运行组", Category: "常规", RiskLevel: "medium",
			MetricUnit: "ppm", EffectiveAt: now, RelatedCode: device,
			PermitRuleCode: rule.Code, PermitRuleVersion: rule.Version,
		}
		revision := &model.DecisionRevision{
			Version: 1, State: state, Evidence: "seed", Reason: "seed decision",
			PermitRuleCode: rule.Code, PermitRuleVersion: rule.Version,
			Actor: "system", RequestID: "seed", CreatedAt: now,
		}
		if err := f.decisions.CreateWithRevision(ctx, &decision, revision); err != nil {
			t.Fatalf("seed decision: %v", err)
		}
	}
	return rule
}

func TestRuleRetirementSucceedsWhenVerificationPasses(t *testing.T) {
	f := newRetirementFixture(t, "file:retire-ok?mode=memory&cache=shared")
	rule := seedRetirementWorld(t, f, "DEVOK", nil)
	ctx := context.Background()

	check, err := f.retirement.Precheck(ctx, rule.ID)
	if err != nil {
		t.Fatalf("precheck: %v", err)
	}
	if !check.Allowed || len(check.Blockers) != 0 || check.DeviceCode != "CU-DEVOK" || check.SampleCode != "ES-DEVOK" {
		t.Fatalf("expected clean check, got %+v", check)
	}

	retired, err := f.retirement.Retire(ctx, rule.ID, rule.Version, model.RoleAdmin, "admin", "req-retire", "装置核验通过，作废旧规则")
	if err != nil {
		t.Fatalf("retire: %v", err)
	}
	if retired.Status != "retired" || retired.Version != rule.Version+1 {
		t.Fatalf("rule not retired as expected: %+v", retired)
	}

	// Rule row is preserved as retired (old rules retain history), not deleted.
	again, err := f.rules.Get(ctx, rule.ID)
	if err != nil || again.Status != "retired" {
		t.Fatalf("retired rule must remain readable: %+v err=%v", again, err)
	}

	// The generic transition endpoint can never move a rule to retired.
	if _, err := f.rule.Transition(ctx, rule.ID, dto.TransitionRequest{
		Status: "superseded", ExpectedVersion: retired.Version, Reason: "attempt to move a retired rule",
	}, "admin", "req-transition"); err == nil {
		t.Fatal("expected retired rule transitions to be rejected")
	}
}

func TestRuleRetirementListsInFlightDecisionBlockers(t *testing.T) {
	f := newRetirementFixture(t, "file:retire-blocked?mode=memory&cache=shared")
	rule := seedRetirementWorld(t, f, "DEVBLOCK", []string{"draft", "review", "accepted"})
	ctx := context.Background()

	check, err := f.retirement.Precheck(ctx, rule.ID)
	if err != nil {
		t.Fatalf("precheck: %v", err)
	}
	if check.Allowed {
		t.Fatal("expected retirement to be blocked")
	}
	var inFlight []RetirementBlocker
	for _, blocker := range check.Blockers {
		if blocker.Code == RetirementBlockInFlight {
			inFlight = append(inFlight, blocker)
		}
	}
	if len(inFlight) != 2 {
		t.Fatalf("expected exactly draft/review blockers, got %+v", check.Blockers)
	}
	codes := map[string]bool{}
	for _, blocker := range inFlight {
		if blocker.DecisionCode == "" || !strings.Contains(blocker.Reason, blocker.DecisionCode) {
			t.Fatalf("blocker must carry 决定编号 and reason: %+v", blocker)
		}
		codes[blocker.DecisionCode] = true
	}
	if !codes["CD-DEVBLOCK-1"] || !codes["CD-DEVBLOCK-2"] {
		t.Fatalf("accepted decision CD-DEVBLOCK-3 must not block, got %v", codes)
	}

	_, err = f.retirement.Retire(ctx, rule.ID, rule.Version, model.RoleAdmin, "admin", "req-denied", "尝试带在途决定作废")
	var blocked *RetirementBlockError
	if !errors.As(err, &blocked) || len(blocked.Blockers) != 2 {
		t.Fatalf("expected structured RetirementBlockError, got %v", err)
	}
	after, _ := f.rules.Get(ctx, rule.ID)
	if after.Status != "active" {
		t.Fatalf("failed retirement must not leave a half-updated rule, status=%s", after.Status)
	}

	// Non-admins cannot retire even when verification would pass.
	other := newRetirementFixture(t, "file:retire-rbac?mode=memory&cache=shared")
	cleanRule := seedRetirementWorld(t, other, "DEVRBAC", nil)
	for _, role := range []string{model.RoleViewer, model.RoleOperator, model.RoleReviewer} {
		if _, err := other.retirement.Retire(ctx, cleanRule.ID, cleanRule.Version, role, role, "req", "非管理员作废"); !errors.Is(err, ErrAdminRequired) {
			t.Fatalf("role %s must be rejected, got %v", role, err)
		}
	}
}

func TestRuleRetirementRejectsMissingDeviceSampleAndMismatch(t *testing.T) {
	f := newRetirementFixture(t, "file:retire-missing?mode=memory&cache=shared")
	ctx := context.Background()
	now := time.Now().UTC()

	missingDevice := model.PermitRule{
		BaseModel: model.BaseModel{Code: "PR-NONE", Name: "无装置规则", Status: "active", Version: 1},
		Facility:  "厂区A", RelatedCode: "GHOST", EffectiveAt: now,
	}
	if err := f.rules.Create(ctx, &missingDevice); err != nil {
		t.Fatalf("seed: %v", err)
	}
	check, err := f.retirement.Precheck(ctx, missingDevice.ID)
	if err != nil {
		t.Fatalf("precheck: %v", err)
	}
	if check.Allowed || !hasBlocker(check.Blockers, RetirementBlockDeviceMissing) {
		t.Fatalf("missing device must block: %+v", check.Blockers)
	}

	missingSample := seedRetirementWorld(t, f, "NOSAMPLE", nil)
	if err := f.db.Delete(&model.EmissionSample{}, "related_code = ?", "NOSAMPLE").Error; err != nil {
		t.Fatalf("remove sample: %v", err)
	}
	check, _ = f.retirement.Precheck(ctx, missingSample.ID)
	if !hasBlocker(check.Blockers, RetirementBlockSampleMissing) {
		t.Fatalf("missing verified sample must block: %+v", check.Blockers)
	}

	mismatch := seedRetirementWorld(t, f, "MISMATCH", nil)
	unit, _ := f.units.FindByRelatedCode(ctx, "MISMATCH")
	unit.Facility = "完全不同的厂区"
	if err := f.db.Save(&unit).Error; err != nil {
		t.Fatalf("mutate facility: %v", err)
	}
	check, _ = f.retirement.Precheck(ctx, mismatch.ID)
	if !hasBlocker(check.Blockers, RetirementBlockDeviceMismatch) {
		t.Fatalf("facility mismatch must block: %+v", check.Blockers)
	}

	draftRule := model.PermitRule{
		BaseModel: model.BaseModel{Code: "PR-DRAFT", Name: "草稿规则", Status: "draft", Version: 1},
		Facility:  "厂区A", RelatedCode: "", EffectiveAt: now,
	}
	if err := f.rules.Create(ctx, &draftRule); err != nil {
		t.Fatalf("seed draft: %v", err)
	}
	check, _ = f.retirement.Precheck(ctx, draftRule.ID)
	if check.Allowed || !hasBlocker(check.Blockers, RetirementBlockRuleState) || !hasBlocker(check.Blockers, RetirementBlockDeviceMissing) {
		t.Fatalf("draft rule with no device must list state + device blockers: %+v", check.Blockers)
	}
}

func TestRetirementReadFailureIsABlocker(t *testing.T) {
	// Migrate only rules so any device/sample/decision query fails at the
	// database level; such read failures must refuse retirement rather than
	// being treated as "nothing found".
	db, err := gorm.Open(sqlite.Open("file:retire-readfail?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.PermitRule{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	coordinator := repository.NewRetirementCoordinator(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	svc := NewRuleRetirementService(repository.NewPermitRuleRepository(db), coordinator, security, NewDeviceLocker())
	ctx := context.Background()
	rule := model.PermitRule{
		BaseModel: model.BaseModel{Code: "PR-BROKEN", Name: "读取失败规则", Status: "active", Version: 1},
		Facility:  "厂区A", RelatedCode: "BROKEN", EffectiveAt: time.Now().UTC(),
	}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, retireErr := svc.Retire(ctx, rule.ID, 1, model.RoleAdmin, "admin", "req", "读取失败时尝试作废")
	var blocked *RetirementBlockError
	if errors.As(retireErr, &blocked) {
		t.Fatalf("infra read failures must surface as errors, not soft blockers: %v", retireErr)
	}
	if retireErr == nil {
		t.Fatal("retirement with failing reads must be rejected")
	}
}

func TestConcurrentRetireAndDecisionSubmitOnlyOneSucceeds(t *testing.T) {
	f := newRetirementFixture(t, "file:retire-race?mode=memory&cache=shared&_pragma=busy_timeout(8000)")
	rule := seedRetirementWorld(t, f, "RACER", nil)
	ctx := context.Background()
	now := time.Now().UTC()

	const workers = 8
	var wg sync.WaitGroup
	results := make(chan string, workers*2)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, err := f.retirement.Retire(ctx, rule.ID, rule.Version, model.RoleAdmin, "admin",
				fmt.Sprintf("req-retire-%d", index), "并发作废")
			if err == nil {
				results <- "retired"
			} else {
				results <- "retire-failed"
			}
		}(i)
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, err := f.decision.Create(ctx, dto.CreateComplianceDecision{
				Code: fmt.Sprintf("CD-RACE-%d", index), Name: "并发决定", Facility: "厂区A", Owner: "运行组",
				Category: "常规", RiskLevel: "low", MetricUnit: "ppm", EffectiveAt: now,
				Evidence: "concurrent", RelatedCode: "RACER",
			}, "operator", fmt.Sprintf("req-decision-%d", index))
			if err == nil {
				results <- "decision"
			} else {
				results <- "decision-failed"
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	counts := map[string]int{}
	for result := range results {
		counts[result]++
	}

	finalRule, _ := f.rules.Get(ctx, rule.ID)
	var decisions []model.ComplianceDecision
	_ = f.db.Where("related_code = ?", "RACER").Find(&decisions).Error

	ruleRetired := finalRule.Status == "retired"
	decisionCreated := len(decisions) > 0
	if ruleRetired && decisionCreated {
		t.Fatalf("both sides committed: rule=%s decisions=%d — half-update violation", finalRule.Status, len(decisions))
	}
	if !ruleRetired && !decisionCreated {
		t.Fatalf("exactly one side must succeed, counts=%v", counts)
	}
	if ruleRetired {
		if counts["retired"] != 1 {
			t.Fatalf("only one retirement may commit, counts=%v", counts)
		}
		if counts["decision"] != 0 {
			t.Fatalf("no decision can be created against a concurrently retired rule, counts=%v", counts)
		}

		// After retirement, further decision submission is refused: the rule
		// row survives as retired but there is no longer an active rule.
		_, err := f.decision.Create(ctx, dto.CreateComplianceDecision{
			Code: "CD-AFTER", Name: "作废后决定", Facility: "厂区A", Owner: "运行组",
			Category: "常规", RiskLevel: "low", MetricUnit: "ppm", EffectiveAt: now,
			Evidence: "late", RelatedCode: "RACER",
		}, "operator", "req-after-decision")
		if !errors.Is(err, ErrNoActivePermitRule) {
			t.Fatalf("creating a decision after retirement must fail, got %v", err)
		}
	} else {
		if counts["decision"] == 0 {
			t.Fatalf("when retirement loses, at least one decision submit must win, counts=%v", counts)
		}
		// The winning draft decisions are now in flight on the device, so a
		// follow-up retirement must be refused listing their 决定编号 — the
		// submit's commit is fully visible and the retire leaves no half-update.
		_, err := f.retirement.Retire(ctx, rule.ID, finalRule.Version, model.RoleAdmin, "admin", "req-after", "决定提交后的后续作废")
		var blocked *RetirementBlockError
		if !errors.As(err, &blocked) || len(blocked.Blockers) != counts["decision"] {
			t.Fatalf("follow-up retirement must be blocked by every in-flight decision, got %v (blockers=%v)", err, blocked)
		}
		for _, blocker := range blocked.Blockers {
			if blocker.Code != RetirementBlockInFlight || blocker.DecisionCode == "" {
				t.Fatalf("blocker must name the in-flight 决定编号: %+v", blocker)
			}
		}
	}
}

func TestDecisionCreationSnapshotsActiveRuleAndRejectsNone(t *testing.T) {
	f := newRetirementFixture(t, "file:retire-snapshot?mode=memory&cache=shared")
	rule := seedRetirementWorld(t, f, "SNAP", nil)
	ctx := context.Background()
	now := time.Now().UTC()

	created, err := f.decision.Create(ctx, dto.CreateComplianceDecision{
		Code: "CD-SNAP-1", Name: "快照决定", Facility: "厂区A", Owner: "运行组",
		Category: "常规", RiskLevel: "low", MetricUnit: "ppm", EffectiveAt: now,
		Evidence: "snapshot test", RelatedCode: "SNAP",
	}, "operator", "req-snap")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.PermitRuleCode != rule.Code || created.PermitRuleVersion != rule.Version {
		t.Fatalf("snapshot mismatch: %+v", created)
	}

	// A draft decision is in flight, so retiring now must be refused with its
	// 决定编号 listed.
	_, err = f.retirement.Retire(ctx, rule.ID, rule.Version, model.RoleAdmin, "admin", "req-early-retire", "决定草稿期尝试作废")
	var blocked *RetirementBlockError
	if !errors.As(err, &blocked) || !hasInFlightBlockerFor(blocked.Blockers, "CD-SNAP-1") {
		t.Fatalf("draft decision must block retirement, got %v", err)
	}

	// Drive the decision to accepted; accepted history never blocks retirement.
	reviewed, err := f.decision.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: 1, Reason: "submit for review",
	}, "operator", model.RoleOperator, "req-review")
	if err != nil {
		t.Fatalf("to review: %v", err)
	}
	accepted, err := f.decision.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "accepted", ExpectedVersion: reviewed.Version, Reason: "accept against pinned permit version",
	}, "reviewer", model.RoleReviewer, "req-accept")
	if err != nil {
		t.Fatalf("to accepted: %v", err)
	}

	if _, err := f.retirement.Retire(ctx, rule.ID, rule.Version, model.RoleAdmin, "admin", "req-retire", "作废后校验历史引用"); err != nil {
		t.Fatalf("retire after acceptance: %v", err)
	}
	// The accepted decision keeps referencing the original version because the
	// retired rule row is retained and the decision snapshot is immutable.
	stored, err := f.decisions.Get(ctx, accepted.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.PermitRuleCode != rule.Code || stored.PermitRuleVersion != rule.Version {
		t.Fatalf("accepted historical decision reference changed: %+v", stored)
	}
	if last := stored.Revisions[len(stored.Revisions)-1]; last.PermitRuleCode != rule.Code || last.PermitRuleVersion != rule.Version {
		t.Fatalf("final revision lost the original permit reference: %+v", last)
	}
}

func hasBlocker(blockers []RetirementBlocker, code string) bool {
	for _, blocker := range blockers {
		if blocker.Code == code {
			return true
		}
	}
	return false
}

func hasInFlightBlockerFor(blockers []RetirementBlocker, code string) bool {
	for _, blocker := range blockers {
		if blocker.Code == RetirementBlockInFlight && blocker.DecisionCode == code {
			return true
		}
	}
	return false
}
