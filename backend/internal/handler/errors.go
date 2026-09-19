package handler

import (
	"errors"
	"net/http"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/repository"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/service"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		util.Fail(c, http.StatusNotFound, "not_found", "record was not found")
	case errors.Is(err, repository.ErrVersionConflict):
		util.Fail(c, http.StatusConflict, "version_conflict", "record changed; refresh and retry")
	case errors.Is(err, service.ErrInvalidTransition), errors.Is(err, service.ErrInvalidInput),
		errors.Is(err, service.ErrReviewerRequired), errors.Is(err, service.ErrDecisionLocked),
		errors.Is(err, service.ErrRetiredTransition), errors.Is(err, service.ErrNoActivePermitRule):
		util.Fail(c, http.StatusUnprocessableEntity, "business_rule", err.Error())
	case errors.Is(err, service.ErrAdminRequired):
		util.Fail(c, http.StatusForbidden, "forbidden", err.Error())
	default:
		_ = c.Error(err)
		util.Fail(c, http.StatusInternalServerError, "internal_error", "request could not be completed")
	}
}

// handleRetirementError maps the verified-retirement failure modes. A block
// error returns 422 together with the structured blockers (each carrying the
// 决定编号 and 阻断原因) so the rule page can list them directly.
func handleRetirementError(c *gin.Context, err error) {
	var blocked *service.RetirementBlockError
	if errors.As(err, &blocked) {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
			"error": "retirement_blocked", "message": blocked.Error(), "details": blocked.Blockers,
		})
		return
	}
	handleError(c, err)
}
