package seeders

import (
	"tachyon-messenger/services/user/models"
)

// Global storage for seeded data to share between seeders
var (
	seededUsers       []*models.User
	seededDepartments []*models.Department
)

// SetUsers stores users for use in other seeders
func SetUsers(users []*models.User) {
	seededUsers = users
}

// GetUsers returns seeded users
func GetUsers() []*models.User {
	return seededUsers
}

// SetDepartments stores departments for use in other seeders
func SetDepartments(departments []*models.Department) {
	seededDepartments = departments
}

// GetDepartments returns seeded departments
func GetDepartments() []*models.Department {
	return seededDepartments
}

// GetRandomUser returns a random user from seeded users
func GetRandomUser() *models.User {
	if len(seededUsers) == 0 {
		return nil
	}
	return seededUsers[randInt(0, len(seededUsers)-1)]
}

// GetRandomUsers returns N random unique users
func GetRandomUsers(n int) []*models.User {
	if n > len(seededUsers) {
		n = len(seededUsers)
	}

	// Create a copy and shuffle
	users := make([]*models.User, len(seededUsers))
	copy(users, seededUsers)

	// Fisher-Yates shuffle
	for i := len(users) - 1; i > 0; i-- {
		j := randInt(0, i)
		users[i], users[j] = users[j], users[i]
	}

	return users[:n]
}

// GetRandomDepartment returns a random department
func GetRandomDepartment() *models.Department {
	if len(seededDepartments) == 0 {
		return nil
	}
	return seededDepartments[randInt(0, len(seededDepartments)-1)]
}
