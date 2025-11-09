package seeders

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"golang.org/x/crypto/bcrypt"
	"tachyon-messenger/services/user/models"
	"tachyon-messenger/shared/database"
	sharedModels "tachyon-messenger/shared/models"
)

func init() {
	gofakeit.Seed(time.Now().UnixNano())
}

// SeedUsers creates departments, subdepartments, and users
func SeedUsers(db *database.DB, userCount int) ([]*models.User, []*models.Department, error) {
	// Russian department names
	departmentNames := []string{
		"Отдел разработки",
		"Отдел маркетинга",
		"Отдел продаж",
		"Бухгалтерия",
		"Отдел кадров",
		"IT-отдел",
		"Отдел поддержки",
		"Юридический отдел",
	}

	subdepartmentMap := map[string][]string{
		"Отдел разработки": {"Frontend", "Backend", "Mobile", "DevOps"},
		"Отдел маркетинга": {"SMM", "Контент", "Аналитика", "PR"},
		"Отдел продаж":     {"B2B", "B2C", "Партнеры"},
		"IT-отдел":         {"Инфраструктура", "Безопасность", "Поддержка"},
	}

	positions := []string{
		"Разработчик", "Старший разработчик", "Тимлид", "Архитектор",
		"Менеджер проекта", "Менеджер по продажам", "Маркетолог",
		"Аналитик", "Дизайнер", "Специалист по поддержке",
		"Бухгалтер", "HR-специалист", "Юрист", "Системный администратор",
	}

	// Russian first names
	maleFirstNames := []string{
		"Александр", "Дмитрий", "Максим", "Сергей", "Андрей",
		"Алексей", "Артём", "Илья", "Кирилл", "Михаил",
		"Иван", "Даниил", "Егор", "Никита", "Владимир",
		"Павел", "Роман", "Денис", "Евгений", "Виктор",
	}

	femaleFirstNames := []string{
		"Анна", "Мария", "Елена", "Ольга", "Наталья",
		"Татьяна", "Ирина", "Екатерина", "Светлана", "Юлия",
		"Дарья", "Анастасия", "Виктория", "Полина", "Александра",
		"Марина", "Алина", "Вера", "Людмила", "София",
	}

	lastNames := []string{
		"Иванов", "Петров", "Сидоров", "Смирнов", "Кузнецов",
		"Попов", "Соколов", "Лебедев", "Козлов", "Новиков",
		"Морозов", "Волков", "Алексеев", "Лебедев", "Семёнов",
		"Егоров", "Павлов", "Козлов", "Степанов", "Николаев",
		"Орлов", "Андреев", "Макаров", "Никитин", "Захаров",
		"Зайцев", "Соловьёв", "Борисов", "Яковлев", "Григорьев",
	}

	// Male patronymics base names
	patronymicBases := []string{
		"Александр", "Дмитрий", "Сергей", "Андрей", "Алексей",
		"Михаил", "Иван", "Владимир", "Павел", "Роман",
		"Евгений", "Виктор", "Николай", "Игорь", "Олег",
	}

	var departments []*models.Department
	var users []*models.User

	// Create departments with subdepartments
	for _, name := range departmentNames {
		dept := &models.Department{
			Name: name,
		}

		if err := db.DB.Create(dept).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to create department %s: %w", name, err)
		}

		departments = append(departments, dept)

		// Create subdepartments if exists
		if subdepts, ok := subdepartmentMap[name]; ok {
			for _, subName := range subdepts {
				subdept := &models.Subdepartment{
					Name:         subName,
					DepartmentID: dept.ID,
				}
				if err := db.DB.Create(subdept).Error; err != nil {
					return nil, nil, fmt.Errorf("failed to create subdepartment %s: %w", subName, err)
				}
			}
		}
	}

	// Create super admin
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	superAdmin := &models.User{
		Email:          "admin@taxion.ru",
		Name:           "Администратор",
		FirstName:      "Иван",
		LastName:       "Администраторов",
		MiddleName:     "Петрович",
		HashedPassword: stringPtr(string(hashedPassword)),
		Role:           sharedModels.RoleSuperAdmin,
		Status:         sharedModels.StatusOnline,
		DepartmentID:   &departments[0].ID,
		Phone:          "+7 (999) 123-45-67",
		Position:       "CTO",
		BirthDate:      timePtr(gofakeit.DateRange(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))),
		IsActive:       true,
	}

	if err := db.DB.Create(superAdmin).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create super admin: %w", err)
	}
	users = append(users, superAdmin)

	// Set department heads
	for i, dept := range departments {
		if i == 0 {
			continue // Skip first department (already has super admin)
		}

		firstName, lastName, middleName, isMale := generateRussianName(maleFirstNames, femaleFirstNames, lastNames, patronymicBases)

		head := &models.User{
			Email:          strings.ToLower(fmt.Sprintf("head.%s@taxion.ru", transliterate(dept.Name))),
			Name:           fmt.Sprintf("%s %s", firstName, lastName),
			FirstName:      firstName,
			LastName:       lastName,
			MiddleName:     middleName,
			HashedPassword: stringPtr(string(hashedPassword)),
			Role:           sharedModels.RoleDepartmentHead,
			Status:         randomUserStatus(),
			DepartmentID:   &dept.ID,
			Phone:          fmt.Sprintf("+7 (9%02d) %03d-%02d-%02d", randInt(10, 99), randInt(100, 999), randInt(10, 99), randInt(10, 99)),
			Position:       "Руководитель отдела",
			BirthDate:      timePtr(gofakeit.DateRange(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))),
			Avatar:         generateAvatarURL(firstName, lastName, isMale),
			IsActive:       true,
		}

		if err := db.DB.Create(head).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to create department head: %w", err)
		}

		// Update department head
		dept.HeadID = &head.ID
		db.DB.Save(dept)

		users = append(users, head)
	}

	// Create regular users
	usersPerDept := (userCount - len(users)) / len(departments)
	for _, dept := range departments {
		for i := 0; i < usersPerDept; i++ {
			firstName, lastName, middleName, isMale := generateRussianName(maleFirstNames, femaleFirstNames, lastNames, patronymicBases)

			user := &models.User{
				Email:          gofakeit.Email(),
				Name:           fmt.Sprintf("%s %s", firstName, lastName),
				FirstName:      firstName,
				LastName:       lastName,
				MiddleName:     middleName,
				HashedPassword: stringPtr(string(hashedPassword)),
				Role:           randomRole(),
				Status:         randomUserStatus(),
				DepartmentID:   &dept.ID,
				Phone:          fmt.Sprintf("+7 (9%02d) %03d-%02d-%02d", randInt(10, 99), randInt(100, 999), randInt(10, 99), randInt(10, 99)),
				Position:       positions[randInt(0, len(positions)-1)],
				BirthDate:      timePtr(gofakeit.DateRange(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))),
				Avatar:         generateAvatarURL(firstName, lastName, isMale),
				IsActive:       true,
			}

			// 30% chance to have a subdepartment
			if rand.Float32() < 0.3 {
				var subdepts []models.Subdepartment
				db.DB.Where("department_id = ?", dept.ID).Find(&subdepts)
				if len(subdepts) > 0 {
					subdept := subdepts[randInt(0, len(subdepts)-1)]
					user.SubdepartmentID = &subdept.ID
				}
			}

			if err := db.DB.Create(user).Error; err != nil {
				return nil, nil, fmt.Errorf("failed to create user: %w", err)
			}

			users = append(users, user)
		}
	}

	return users, departments, nil
}

func randomRole() sharedModels.Role {
	roles := []sharedModels.Role{
		sharedModels.RoleEmployee,
		sharedModels.RoleEmployee,
		sharedModels.RoleEmployee,
		sharedModels.RoleAdmin,
	}
	return roles[randInt(0, len(roles)-1)]
}

func randomUserStatus() sharedModels.UserStatus {
	statuses := []sharedModels.UserStatus{
		sharedModels.StatusOnline,
		sharedModels.StatusOnline,
		sharedModels.StatusBusy,
		sharedModels.StatusAway,
		sharedModels.StatusOffline,
	}
	return statuses[randInt(0, len(statuses)-1)]
}

func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func randInt(min, max int) int {
	return min + rand.Intn(max-min+1)
}

// generateRussianName generates a random Russian name
func generateRussianName(maleFirstNames, femaleFirstNames, lastNames, patronymicBases []string) (firstName, lastName, middleName string, isMale bool) {
	isMale = rand.Float32() < 0.6 // 60% male, 40% female

	if isMale {
		firstName = maleFirstNames[randInt(0, len(maleFirstNames)-1)]
		lastName = lastNames[randInt(0, len(lastNames)-1)]
	} else {
		firstName = femaleFirstNames[randInt(0, len(femaleFirstNames)-1)]
		lastName = lastNames[randInt(0, len(lastNames)-1)] + "а" // Add "а" for female last names
	}

	// Generate patronymic (middle name)
	patronymicBase := patronymicBases[randInt(0, len(patronymicBases)-1)]
	middleName = generatePatronymic(patronymicBase, isMale)

	return firstName, lastName, middleName, isMale
}

// generatePatronymic creates a Russian patronymic from a father's first name
func generatePatronymic(fatherName string, isMale bool) string {
	// Remove common endings and add patronymic suffix
	endings := map[string]string{
		"Александр": "Александр",
		"Дмитрий":   "Дмитриев",
		"Сергей":    "Сергеев",
		"Андрей":    "Андреев",
		"Алексей":   "Алексеев",
		"Михаил":    "Михайл",
		"Иван":      "Иван",
		"Владимир":  "Владимир",
		"Павел":     "Павл",
		"Роман":     "Роман",
		"Евгений":   "Евгениев",
		"Виктор":    "Виктор",
		"Николай":   "Николаев",
		"Игорь":     "Игорев",
		"Олег":      "Олег",
	}

	base := endings[fatherName]
	if base == "" {
		base = fatherName
	}

	if isMale {
		return base + "ович"
	}
	return base + "овна"
}

// generateAvatarURL creates an avatar URL
func generateAvatarURL(firstName, lastName string, isMale bool) string {
	// UI Faces API - real professional photos
	// Available IDs range from 1 to 200+ (using 1-150 for variety)
	photoNumber := randInt(1, 150)
	return fmt.Sprintf("https://mockmind-api.uifaces.co/content/human/%d.jpg", photoNumber)
}

// Simple transliteration for department names
func transliterate(s string) string {
	translitMap := map[rune]string{
		'А': "A", 'Б': "B", 'В': "V", 'Г': "G", 'Д': "D", 'Е': "E", 'Ё': "Yo", 'Ж': "Zh",
		'З': "Z", 'И': "I", 'Й': "Y", 'К': "K", 'Л': "L", 'М': "M", 'Н': "N", 'О': "O",
		'П': "P", 'Р': "R", 'С': "S", 'Т': "T", 'У': "U", 'Ф': "F", 'Х': "Kh", 'Ц': "Ts",
		'Ч': "Ch", 'Ш': "Sh", 'Щ': "Shch", 'Ъ': "", 'Ы': "Y", 'Ь': "", 'Э': "E", 'Ю': "Yu", 'Я': "Ya",
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo", 'ж': "zh",
		'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
		'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts",
		'ч': "ch", 'ш': "sh", 'щ': "shch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
		' ': "_", '-': "_",
	}

	result := ""
	for _, r := range s {
		if t, ok := translitMap[r]; ok {
			result += t
		} else {
			result += string(r)
		}
	}
	return result
}
