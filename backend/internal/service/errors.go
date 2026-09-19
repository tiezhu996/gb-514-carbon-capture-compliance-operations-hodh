package service

import "errors"

var (
	ErrInvalidTransition  = errors.New("requested status transition is not allowed")
	ErrInvalidInput       = errors.New("business input validation failed")
	ErrUnauthorized       = errors.New("invalid username or password")
	ErrInactiveUser       = errors.New("user account is inactive")
	ErrReviewerRequired   = errors.New("reviewer or admin role is required for this decision")
	ErrDecisionLocked     = errors.New("compliance decision fields are locked after review begins")
	ErrRetirementBlocked  = errors.New("permit rule retirement is blocked by device verification")
	ErrAdminRequired      = errors.New("admin role is required to retire a permit rule")
	ErrRetiredTransition  = errors.New("retired permit rules are terminal; use the verified retirement action")
	ErrNoActivePermitRule = errors.New("no active permit rule is referenced by this device")
)
