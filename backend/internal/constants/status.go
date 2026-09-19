package constants

// Shared status values are mirrored in frontend/src/types/status.ts. Keeping
// the lists explicit makes state-machine drift visible during code review.

type UnitState string

const (
	UnitStateStandby UnitState = "standby"
	UnitStateRunning UnitState = "running"
	UnitStateLimited UnitState = "limited"
	UnitStateStopped UnitState = "stopped"
)

var AllUnitState = []string{"standby", "running", "limited", "stopped"}

// PermitRuleState names the 许可规则 lifecycle states. Retirement (retired) is
// a terminal state reached only through the verified RuleRetirementService.
type PermitRuleState string

const (
	PermitRuleStateDraft      PermitRuleState = "draft"
	PermitRuleStateActive     PermitRuleState = "active"
	PermitRuleStateSuperseded PermitRuleState = "superseded"
	PermitRuleStateRetired    PermitRuleState = "retired"
)

type DecisionState string

const (
	DecisionStateDraft     DecisionState = "draft"
	DecisionStateReview    DecisionState = "review"
	DecisionStateAccepted  DecisionState = "accepted"
	DecisionStateEscalated DecisionState = "escalated"
)

var AllDecisionState = []string{"draft", "review", "accepted", "escalated"}

var CaptureUnitTransitions = map[string]map[string]bool{
	"standby": {"running": true, "limited": true},
	"running": {"limited": true, "stopped": true, "standby": true},
	"limited": {"stopped": true, "running": true},
	"stopped": {"limited": true},
}

var PermitRuleTransitions = map[string]map[string]bool{
	"draft":      {"active": true, "superseded": true},
	"active":     {"superseded": true, "retired": true, "draft": true},
	"superseded": {"retired": true, "active": true},
	// retired is terminal: verified retirement preserves the old rule for
	// history and accepted decisions, and it must never flow back to active
	// and bypass the pre-retirement device verification.
	"retired": {},
}

var EmissionSampleTransitions = map[string]map[string]bool{
	"collected": {"testing": true, "verified": true},
	"testing":   {"verified": true, "invalid": true, "collected": true},
	"verified":  {"invalid": true, "testing": true},
	"invalid":   {"verified": true},
}

var ComplianceDecisionTransitions = map[string]map[string]bool{
	"draft":     {"review": true},
	"review":    {"accepted": true, "escalated": true, "draft": true},
	"accepted":  {"escalated": true, "review": true},
	"escalated": {"accepted": true},
}

func CanTransition(graph map[string]map[string]bool, from, to string) bool {
	targets, exists := graph[from]
	return exists && targets[to]
}
