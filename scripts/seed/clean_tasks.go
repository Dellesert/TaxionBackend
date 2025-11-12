package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"tachyon-messenger/scripts/seed/seeders"
	"tachyon-messenger/shared/database"
)

func main() {
	fmt.Println("🗑️  Скрипт удаления моковых задач")
	fmt.Println("=====================================")

	// Загружаем переменные окружения
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  Предупреждение: .env файл не найден")
	}

	// Подключаемся к базе данных
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("❌ Ошибка: переменная окружения DATABASE_URL обязательна")
	}

	dbConfig := database.ConfigFromEnv(dsn)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("❌ Не удалось подключиться к базе данных: %v", err)
	}

	fmt.Println("✅ Подключение к базе данных установлено")
	fmt.Println()

	// Удаляем все задачи
	if err := seeders.CleanTasks(db); err != nil {
		log.Fatalf("❌ Ошибка при удалении задач: %v", err)
	}

	fmt.Println()
	fmt.Println("✨ Готово! Все моковые задачи удалены из базы данных.")
	fmt.Println("📝 Примечание: Пользователи, чаты, опросы и события календаря остались нетронутыми.")
}
