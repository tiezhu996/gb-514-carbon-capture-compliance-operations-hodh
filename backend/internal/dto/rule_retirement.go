package dto

// RetirePermitRule is the write contract for the verified 许可规则 retirement
// action. The optimistic-lock version is required so a stale rule page cannot
// retire a rule changed by another request.
type RetirePermitRule struct {
	ExpectedVersion uint   `json:"expectedVersion" binding:"required"`
	Reason          string `json:"reason" binding:"required,min=3,max=500"`
}
