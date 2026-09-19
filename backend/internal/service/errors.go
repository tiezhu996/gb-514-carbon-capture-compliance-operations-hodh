package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
)

var (
	ErrInvalidTransition = errors.New("requested status transition is not allowed")
	ErrInvalidInput      = errors.New("business input validation failed")
	ErrUnauthorized      = errors.New("invalid username or password")
	ErrInactiveUser      = errors.New("user account is inactive")
	ErrReviewerRequired  = errors.New("reviewer or admin role is required for this decision")
	ErrDecisionLocked    = errors.New("compliance decision fields are locked after review begins")
	ErrAdminRequired     = errors.New("admin role is required to retire a permit rule")
)

// RetireBlockedError carries every per-unit verification blocker so the API
// can list decision codes and reasons on the rule page instead of failing
// with an opaque message.
type RetireBlockedError struct {
	Blockers []dto.RetireBlocker
}

func (e *RetireBlockedError) Error() string {
	parts := make([]string, 0, len(e.Blockers))
	for _, blocker := range e.Blockers {
		if blocker.DecisionCode != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", blocker.DecisionCode, blocker.Reason))
			continue
		}
		parts = append(parts, blocker.Reason)
	}
	return "permit rule retire verification failed: " + strings.Join(parts, "; ")
}
