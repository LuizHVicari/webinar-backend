package user

type JoinViaInviteRequest struct {
	InviteID string `json:"invite_id" binding:"required,uuid"`
}

type CreateOrgRequest struct {
	OrgName string `json:"org_name" binding:"required,min=1,max=255"`
}

type ChangeRoleRequest struct {
	Role string `json:"role" binding:"required"`
}
