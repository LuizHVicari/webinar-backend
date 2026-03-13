package organization

type Role string

const (
	RoleAdmin         Role = "admin"
	RoleManager       Role = "manager"
	RoleHumanResource Role = "human-resource"
	RoleDeveloper     Role = "developer"
)

var allRoles = []Role{RoleAdmin, RoleHumanResource, RoleManager, RoleDeveloper}

func (r Role) IsValid() bool {
	for _, v := range allRoles {
		if r == v {
			return true
		}
	}
	return false
}

func (r Role) IsAdminOrHR() bool {
	return r == RoleAdmin || r == RoleHumanResource
}

// Ordered from highest to lowest privilege, used for role-resolution checks.
func Roles() []Role {
	return allRoles
}
