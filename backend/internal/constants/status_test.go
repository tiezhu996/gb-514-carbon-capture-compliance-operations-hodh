package constants

import "testing"

func TestCaptureUnitTransitionGraph(t *testing.T) {
	if !CanTransition(CaptureUnitTransitions, "standby", "running") {
		t.Fatalf("expected standby -> running transition to be allowed")
	}
	if CanTransition(CaptureUnitTransitions, "standby", "unknown") {
		t.Fatal("unknown status must never be accepted")
	}
}

func TestComplianceDecisionRequiresReviewBeforeAcceptance(t *testing.T) {
	if !CanTransition(ComplianceDecisionTransitions, "draft", "review") {
		t.Fatal("draft decisions must be submittable for review")
	}
	if CanTransition(ComplianceDecisionTransitions, "draft", "accepted") {
		t.Fatal("draft decisions must not skip independent review")
	}
}
