package organization

type CreateInviteRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role"  binding:"required"`
}
