package dto

import "time"

// CreatePermitRule is the public write contract for 许可规则. Status is deliberately
// omitted so callers cannot bypass the service state machine.
type CreatePermitRule struct {
	Code        string    `json:"code" binding:"required,min=2,max=64"`
	Name        string    `json:"name" binding:"required,min=2,max=160"`
	Description string    `json:"description" binding:"max=1000"`
	Facility    string    `json:"facility" binding:"required,max=120"`
	Owner       string    `json:"owner" binding:"required,max=120"`
	Category    string    `json:"category" binding:"required,max=80"`
	RiskLevel   string    `json:"riskLevel" binding:"required,oneof=low medium high critical"`
	MetricValue float64   `json:"metricValue"`
	MetricUnit  string    `json:"metricUnit" binding:"max=24"`
	EffectiveAt time.Time `json:"effectiveAt" binding:"required"`
	Evidence    string    `json:"evidence" binding:"max=2000"`
	RelatedCode string    `json:"relatedCode" binding:"max=64"`
}

type UpdatePermitRule struct {
	ExpectedVersion uint      `json:"expectedVersion" binding:"required"`
	Name            string    `json:"name" binding:"required,min=2,max=160"`
	Description     string    `json:"description" binding:"max=1000"`
	Facility        string    `json:"facility" binding:"required,max=120"`
	Owner           string    `json:"owner" binding:"required,max=120"`
	Category        string    `json:"category" binding:"required,max=80"`
	RiskLevel       string    `json:"riskLevel" binding:"required,oneof=low medium high critical"`
	MetricValue     float64   `json:"metricValue"`
	MetricUnit      string    `json:"metricUnit" binding:"max=24"`
	EffectiveAt     time.Time `json:"effectiveAt" binding:"required"`
	Evidence        string    `json:"evidence" binding:"max=2000"`
	RelatedCode     string    `json:"relatedCode" binding:"max=64"`
}

// RetirePermitRule is the 作废 write contract. ExpectedVersion keeps the
// optimistic-lock semantics shared with every other write operation.
type RetirePermitRule struct {
	ExpectedVersion uint   `json:"expectedVersion" binding:"required"`
	Reason          string `json:"reason" binding:"required,min=3,max=500"`
}

// RetireBlocker is one verification failure. DecisionCode is populated when an
// in-flight 合规决定 blocks the retire so the rule page can list it directly.
type RetireBlocker struct {
	DecisionCode string `json:"decisionCode,omitempty"`
	Reason       string `json:"reason"`
}

// RuleRetireCheck is the read-only per-unit verification result that the rule
// page renders before an administrator attempts the retire.
type RuleRetireCheck struct {
	RuleID    uint            `json:"ruleId"`
	RuleCode  string          `json:"ruleCode"`
	Allowed   bool            `json:"allowed"`
	Blockers  []RetireBlocker `json:"blockers"`
	CheckedAt time.Time       `json:"checkedAt"`
}
