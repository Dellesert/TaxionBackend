package seeders

import (
	"fmt"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	chatModels "tachyon-messenger/services/chat/models"
	userModels "tachyon-messenger/services/user/models"
	"tachyon-messenger/shared/database"
)

// SeedChats creates group chats, private chats, and messages
func SeedChats(db *database.DB, chatCount int) ([]*chatModels.Chat, error) {
	users := GetUsers()
	if len(users) == 0 {
		return nil, fmt.Errorf("no users found, please seed users first")
	}

	var chats []*chatModels.Chat

	// Russian chat topics for group chats
	groupChatTopics := []string{
		"Общий чат",
		"Обсуждение проекта",
		"Планирование спринта",
		"Технические вопросы",
		"Обеденный перерыв",
		"Вопросы по задачам",
		"Новости компании",
		"HR и объявления",
		"Отдел разработки",
		"Маркетинговая команда",
		"Поддержка клиентов",
		"Идеи и предложения",
		"Релиз v2.0",
		"Годовой отчет",
		"Корпоратив",
	}

	messageTemplates := []string{
		"Добрый день! Как дела с задачей?",
		"Нужна помощь с %s",
		"Кто может посмотреть на это?",
		"Отличная работа, команда!",
		"Давайте обсудим это на встрече",
		"Согласен с предыдущим комментарием",
		"Есть обновления по этому вопросу?",
		"Спасибо за быстрый ответ!",
		"Я сейчас работаю над этим",
		"Завтра предоставлю результаты",
		"Отлично, продолжаем в том же духе",
		"Может кто-то помочь?",
		"Это срочно, нужно решить сегодня",
		"Уже готово, можете проверить",
		"Хорошо, учту ваши замечания",
	}

	// Create group chats (60% of total)
	groupChatCount := (chatCount * 60) / 100
	for i := 0; i < groupChatCount; i++ {
		creator := users[randInt(0, len(users)-1)]
		chatName := groupChatTopics[randInt(0, len(groupChatTopics)-1)]

		chat := &chatModels.Chat{
			Name:        chatName,
			Description: gofakeit.Sentence(10),
			Type:        chatModels.ChatTypeGroup,
			CreatorID:   creator.ID,
			Avatar:      generateGroupChatAvatar(chatName),
			IsActive:    true,
		}

		if err := db.DB.Create(chat).Error; err != nil {
			return nil, fmt.Errorf("failed to create group chat: %w", err)
		}

		// Add 3-10 members to group chat (excluding creator - will be added by AfterCreate hook)
		memberCount := randInt(3, 10)
		allMembers := GetRandomUsers(memberCount)

		// Use map to ensure uniqueness (exclude creator - will be added by AfterCreate hook)
		memberMap := make(map[uint]*userModels.User)

		for _, m := range allMembers {
			if m.ID != creator.ID { // Skip creator to avoid duplicate
				memberMap[m.ID] = m
			}
		}

		// Convert map to slice and add creator first for messages
		members := make([]*userModels.User, 0, len(memberMap)+1)
		members = append(members, creator) // Add creator first for messages
		for _, m := range memberMap {
			members = append(members, m)
		}

		for j, member := range members {
			// Skip creator - already added by AfterCreate hook
			if member.ID == creator.ID {
				continue
			}

			role := chatModels.ChatMemberRoleMember
			if j == 1 && randInt(1, 3) == 1 { // 33% chance for second member to be admin
				role = chatModels.ChatMemberRoleAdmin
			}

			chatMember := &chatModels.ChatMember{
				ChatID:   chat.ID,
				UserID:   member.ID,
				Role:     role,
				JoinedAt: time.Now().Add(-time.Duration(randInt(1, 90)) * 24 * time.Hour),
				IsActive: true,
			}

			// Some members have favorites/pins
			if randInt(1, 4) == 1 {
				chatMember.IsFavorite = true
			}
			if randInt(1, 5) == 1 {
				chatMember.IsPinned = true
			}

			if err := db.DB.Create(chatMember).Error; err != nil {
				return nil, fmt.Errorf("failed to create chat member: %w", err)
			}
		}

		// Create 10-50 messages per chat
		messageCount := randInt(10, 50)
		if err := createMessages(db, chat, members, messageCount, messageTemplates); err != nil {
			return nil, err
		}

		chats = append(chats, chat)
	}

	// Create private chats (40% of total)
	privateChatCount := chatCount - groupChatCount
	for i := 0; i < privateChatCount; i++ {
		// Get two random users
		randomUsers := GetRandomUsers(2)
		if len(randomUsers) < 2 {
			continue
		}

		user1 := randomUsers[0]
		user2 := randomUsers[1]

		chat := &chatModels.Chat{
			Name:      fmt.Sprintf("%s, %s", user1.Name, user2.Name),
			Type:      chatModels.ChatTypePrivate,
			CreatorID: user1.ID,
			IsActive:  true,
		}

		if err := db.DB.Create(chat).Error; err != nil {
			return nil, fmt.Errorf("failed to create private chat: %w", err)
		}

		// Add both members (skip creator - already added by AfterCreate hook)
		for _, user := range randomUsers {
			// Skip creator - already added by AfterCreate hook
			if user.ID == chat.CreatorID {
				continue
			}

			chatMember := &chatModels.ChatMember{
				ChatID:   chat.ID,
				UserID:   user.ID,
				Role:     chatModels.ChatMemberRoleMember,
				JoinedAt: time.Now().Add(-time.Duration(randInt(1, 60)) * 24 * time.Hour),
				IsActive: true,
			}

			if err := db.DB.Create(chatMember).Error; err != nil {
				return nil, fmt.Errorf("failed to create chat member: %w", err)
			}
		}

		// Create 5-30 messages per private chat
		messageCount := randInt(5, 30)
		if err := createMessages(db, chat, randomUsers, messageCount, messageTemplates); err != nil {
			return nil, err
		}

		chats = append(chats, chat)
	}

	return chats, nil
}

func createMessages(db *database.DB, chat *chatModels.Chat, members []*userModels.User, count int, templates []string) error {
	startTime := time.Now().Add(-time.Duration(randInt(1, 30)) * 24 * time.Hour)

	for i := 0; i < count; i++ {
		sender := members[randInt(0, len(members)-1)]

		// Message type distribution
		var messageType chatModels.MessageType
		var content string

		roll := randInt(1, 100)
		switch {
		case roll <= 80: // 80% text messages
			messageType = chatModels.MessageTypeText
			// Sometimes add technical terms (33% chance)
			if randInt(1, 3) == 1 {
				topics := []string{"API", "базой данных", "фронтендом", "бэкендом", "дизайном", "тестированием", "релизом"}
				content = fmt.Sprintf("Нужна помощь с %s", topics[randInt(0, len(topics)-1)])
			} else {
				// Use templates without placeholders
				simpleTemplates := []string{
					"Добрый день! Как дела с задачей?",
					"Кто может посмотреть на это?",
					"Отличная работа, команда!",
					"Давайте обсудим это на встрече",
					"Согласен с предыдущим комментарием",
					"Есть обновления по этому вопросу?",
					"Спасибо за быстрый ответ!",
					"Я сейчас работаю над этим",
					"Завтра предоставлю результаты",
					"Отлично, продолжаем в том же духе",
					"Может кто-то помочь?",
					"Это срочно, нужно решить сегодня",
					"Уже готово, можете проверить",
					"Хорошо, учту ваши замечания",
				}
				content = simpleTemplates[randInt(0, len(simpleTemplates)-1)]
			}
		case roll <= 90: // 10% image messages
			messageType = chatModels.MessageTypeImage
			content = "Скриншот прикреплен"
		case roll <= 95: // 5% file messages
			messageType = chatModels.MessageTypeFile
			content = "Документ прикреплен"
		default: // 5% system messages
			messageType = chatModels.MessageTypeSystem
			systemMsgs := []string{
				fmt.Sprintf("%s добавил(а) участника", sender.Name),
				fmt.Sprintf("%s изменил(а) название чата", sender.Name),
				fmt.Sprintf("%s покинул(а) чат", sender.Name),
			}
			content = systemMsgs[randInt(0, len(systemMsgs)-1)]
		}

		message := &chatModels.Message{
			ChatID:   chat.ID,
			SenderID: sender.ID,
			Content:  content,
			Type:     messageType,
			Status:   chatModels.MessageStatusRead,
		}

		// 10% chance of being edited
		if randInt(1, 10) == 1 && messageType == chatModels.MessageTypeText {
			message.IsEdited = true
			editTime := message.CreatedAt.Add(time.Duration(randInt(1, 60)) * time.Minute)
			message.EditedAt = &editTime
		}

		// 5% chance of being pinned
		if randInt(1, 20) == 1 {
			message.IsPinned = true
		}

		// 10% chance of reply
		if i > 0 && randInt(1, 10) == 1 {
			replyToID := uint(i) // Previous message
			message.ReplyToID = &replyToID
		}

		if err := db.DB.Create(message).Error; err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}

		// Add reactions (20% chance)
		if randInt(1, 5) == 1 && messageType == chatModels.MessageTypeText {
			reactionEmojis := []string{"👍", "❤️", "😊", "👏", "🔥", "✅"}
			reactorsCount := randInt(1, 3)
			reactors := GetRandomUsers(reactorsCount)

			for _, reactor := range reactors {
				reaction := &chatModels.MessageReaction{
					MessageID: message.ID,
					UserID:    reactor.ID,
					Emoji:     reactionEmojis[randInt(0, len(reactionEmojis)-1)],
				}
				db.DB.Create(reaction)
			}
		}

		// Add read receipts for members
		for _, member := range members {
			if member.ID == sender.ID {
				continue // Skip sender
			}

			// 70% chance the message is read
			if randInt(1, 10) <= 7 {
				readAt := message.CreatedAt.Add(time.Duration(randInt(1, 120)) * time.Minute)
				receipt := &chatModels.MessageReadReceipt{
					MessageID: message.ID,
					UserID:    member.ID,
					ReadAt:    readAt,
				}
				db.DB.Create(receipt)
			}
		}
	}

	// Update chat's last message time
	chat.LastMessageAt = &startTime
	db.DB.Save(chat)

	return nil
}

// generateGroupChatAvatar generates an avatar URL for group chats
// Uses UI Avatars API or similar service to create avatars based on chat name
func generateGroupChatAvatar(chatName string) string {
	// Option 1: UI Avatars - generates colorful avatars with initials
	// Get first letters of first two words for initials
	words := []rune(chatName)
	var initials string

	if len(words) > 0 {
		initials = string(words[0])
		// Find second word
		for i, r := range words {
			if r == ' ' && i+1 < len(words) {
				initials += string(words[i+1])
				break
			}
		}
	}

	// Random background colors for variety
	colors := []string{
		"3b82f6", // blue
		"8b5cf6", // purple
		"ec4899", // pink
		"f59e0b", // amber
		"10b981", // green
		"6366f1", // indigo
		"ef4444", // red
		"14b8a6", // teal
		"f97316", // orange
		"a855f7", // violet
	}

	bgColor := colors[randInt(0, len(colors)-1)]

	// UI Avatars API: https://ui-avatars.com/
	// Format: https://ui-avatars.com/api/?name=John+Doe&background=random&size=128
	return fmt.Sprintf("https://ui-avatars.com/api/?name=%s&background=%s&color=fff&size=256&bold=true&format=png",
		initials, bgColor)
}
