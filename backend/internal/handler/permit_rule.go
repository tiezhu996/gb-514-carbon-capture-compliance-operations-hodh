package handler

import (
	"net/http"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/dto"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/middleware"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/service"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/util"
	"github.com/gin-gonic/gin"
)

type PermitRuleHandler struct{ service service.PermitRuleService }

func NewPermitRuleHandler(s service.PermitRuleService) *PermitRuleHandler {
	return &PermitRuleHandler{service: s}
}

func (h *PermitRuleHandler) Register(group *gin.RouterGroup) {
	resource := group.Group("/rules")
	resource.GET("", h.list)
	resource.GET("/:id", h.get)
	resource.GET("/:id/retire-check", h.retireCheck)
	resource.POST("", middleware.RequireMinimumRole("operator"), h.create)
	resource.PUT("/:id", middleware.RequireMinimumRole("operator"), h.update)
	resource.POST("/:id/transition", middleware.RequireMinimumRole("operator"), h.transition)
	resource.POST("/:id/retire", middleware.RequireRoles("admin"), h.retire)
	resource.DELETE("/:id", middleware.RequireRoles("admin"), h.remove)
}

func (h *PermitRuleHandler) list(c *gin.Context) {
	query := bindPage(c)
	result, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}
	util.Page(c, result.Items, result.Page, result.PageSize, result.Total)
}

func (h *PermitRuleHandler) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *PermitRuleHandler) create(c *gin.Context) {
	var input dto.CreatePermitRule
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Create(c.Request.Context(), input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.Created(c, item)
}

func (h *PermitRuleHandler) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input dto.UpdatePermitRule
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *PermitRuleHandler) transition(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input dto.TransitionRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Transition(c.Request.Context(), id, input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

// retireCheck exposes the per-unit verification so the rule page can list
// blocking decision codes and reasons before anyone attempts the retire.
func (h *PermitRuleHandler) retireCheck(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := h.service.RetireCheck(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, result)
}

// retire voids the rule. The route already requires admin; the service
// re-checks the role and runs the verification transactionally.
func (h *PermitRuleHandler) retire(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input dto.RetirePermitRule
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Retire(c.Request.Context(), id, input, actorFromContext(c), roleFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *PermitRuleHandler) remove(c *gin.Context) {
	if roleFromContext(c) != "admin" {
		util.Fail(c, http.StatusForbidden, "forbidden", "admin role is required")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id, actorFromContext(c), requestIDFromContext(c)); err != nil {
		handleError(c, err)
		return
	}
	util.NoContent(c)
}
