package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "password123"
	// Hash from database
	hash := "$2a$10$kifDiG/6oGbLmnnhHMpCcOJY.OwHCHTphyw0RG1TEP33i8BMWuGza"
	
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		fmt.Println("Password does NOT match:", err)
	} else {
		fmt.Println("Password matches!")
	}
}
