package model

import "time"

// ComplianceDecision models 合规决定 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type ComplianceDecision struct {
	BaseModel
	Facility    string    `json:"facility" gorm:"size:120;index"`
	Owner       string    `json:"owner" gorm:"size:120;index"`
	Category    string    `json:"category" gorm:"size:80;index"`
	RiskLevel   string    `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64   `json:"metricValue"`
	MetricUnit  string    `json:"metricUnit" gorm:"size:24"`
	EffectiveAt time.Time `json:"effectiveAt"`
	Evidence    string    `json:"evidence" gorm:"size:2000"`
	RelatedCode string    `json:"relatedCode" gorm:"size:64;index"`
	// PermitRuleCode/PermitRuleVersion snapshot the active 许可规则 that justified
	// the decision. They are pinned at draft creation so accepted decisions keep
	// referencing the original rule version even after the rule is retired.
	PermitRuleCode    string             `json:"permitRuleCode" gorm:"size:64;index"`
	PermitRuleVersion uint               `json:"permitRuleVersion"`
	Revisions         []DecisionRevision `json:"revisions" gorm:"foreignKey:ComplianceDecisionID;constraint:OnDelete:CASCADE"`
}

func (item *ComplianceDecision) GetBase() *BaseModel { return &item.BaseModel }

func (item ComplianceDecision) TableName() string { return "compliance_decisions" }

var ComplianceDecisionInitialStatus = "draft"

// DecisionRevision is an append-only compliance snapshot. It keeps the
// evidence and request context that justified every aggregate version.
type DecisionRevision struct {
	ID                   uint      `json:"id" gorm:"primaryKey"`
	ComplianceDecisionID uint      `json:"complianceDecisionId" gorm:"uniqueIndex:idx_decision_revision_version;not null"`
	Version              uint      `json:"version" gorm:"uniqueIndex:idx_decision_revision_version;not null"`
	State                string    `json:"state" gorm:"size:40;not null"`
	Evidence             string    `json:"evidence" gorm:"size:2000;not null"`
	Reason               string    `json:"reason" gorm:"size:500;not null"`
	PermitRuleCode       string    `json:"permitRuleCode" gorm:"size:64;not null"`
	PermitRuleVersion    uint      `json:"permitRuleVersion;not null;default:0"`
	Actor                string    `json:"actor" gorm:"size:80;not null;index"`
	RequestID            string    `json:"requestId" gorm:"size:64;not null;index"`
	CreatedAt            time.Time `json:"createdAt" gorm:"index"`
}
