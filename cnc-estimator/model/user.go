package model

import (
	"fmt"
	"strings"
	"time"
)

type User struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (user User) Validate() error {
	if strings.TrimSpace(user.ID) == "" {
		return fmt.Errorf("user ID is required")
	}
	if strings.TrimSpace(user.Email) == "" || !strings.Contains(user.Email, "@") {
		return fmt.Errorf("user email must be valid")
	}
	if strings.TrimSpace(user.Name) == "" {
		return fmt.Errorf("user name is required")
	}
	return nil
}
