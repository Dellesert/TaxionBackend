package usecase

import (
	sharedmodels "tachyon-messenger/shared/models"
)

// isValidRole checks if the role is valid
func isValidRole(role string) bool {
	validRoles := []string{
		string(sharedmodels.RoleSuperAdmin),
		string(sharedmodels.RoleAdmin),
		string(sharedmodels.RoleManager),
		string(sharedmodels.RoleEmployee),
	}

	for _, validRole := range validRoles {
		if role == validRole {
			return true
		}
	}
	return false
}

// isValidStatus checks if the user status is valid
func isValidStatus(status sharedmodels.UserStatus) bool {
	validStatuses := []sharedmodels.UserStatus{
		sharedmodels.StatusOnline,
		sharedmodels.StatusBusy,
		sharedmodels.StatusAway,
		sharedmodels.StatusOffline,
	}

	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}
