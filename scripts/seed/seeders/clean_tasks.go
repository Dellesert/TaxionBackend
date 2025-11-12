package seeders

import (
	"fmt"

	"tachyon-messenger/shared/database"
)

// CleanTasks удаляет все задачи и связанные с ними данные из базы данных
// Оставляет все остальные данные (пользователи, чаты, опросы, события) нетронутыми
func CleanTasks(db *database.DB) error {
	// Порядок важен: сначала дочерние таблицы, затем родительские
	tables := []string{
		"task_checklist_items",  // Элементы чек-листов
		"task_checklists",       // Чек-листы задач
		"task_attachments",      // Вложения задач
		"task_comments",         // Комментарии к задачам
		"task_activities",       // История активности задач
		"task_assignees",        // Назначенные исполнители (many-to-many)
		"tasks",                 // Сами задачи (включая подзадачи)
	}

	fmt.Println("🧹 Удаление моковых задач...")

	for _, table := range tables {
		result := db.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if result.Error != nil {
			// Игнорируем ошибку, если таблица не существует
			errMsg := result.Error.Error()
			if errMsg != fmt.Sprintf("ERROR: relation \"%s\" does not exist (SQLSTATE 42P01)", table) {
				return fmt.Errorf("не удалось очистить таблицу %s: %w", table, result.Error)
			}
			fmt.Printf("⚠️  Таблица %s не существует, пропускаем\n", table)
		} else {
			fmt.Printf("✅ Таблица %s очищена\n", table)
		}
	}

	// Сбрасываем sequence для tasks
	fmt.Println("🔄 Сброс счётчика ID задач...")
	if err := db.DB.Exec("ALTER SEQUENCE tasks_id_seq RESTART WITH 1").Error; err != nil {
		// Игнорируем, если sequence не существует
		errMsg := err.Error()
		if errMsg != "ERROR: relation \"tasks_id_seq\" does not exist (SQLSTATE 42P01)" {
			return fmt.Errorf("не удалось сбросить sequence tasks_id_seq: %w", err)
		}
		fmt.Println("⚠️  Sequence tasks_id_seq не существует, пропускаем")
	} else {
		fmt.Println("✅ Счётчик ID задач сброшен")
	}

	fmt.Println("🎉 Все моковые задачи успешно удалены!")
	return nil
}
