package application

import (
	"encoding/json"
	"time"

	"smetchic/model"
)

type CompanyUserRecord struct {
	User model.User
	Role model.CompanyRole
}

type OrderRecord struct {
	ID          string
	CompanyID   string
	CreatedByID string
	ProjectID   string
	Name        string
	Description string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Attributes  json.RawMessage
}

const (
	OrderStatusDraft      = "draft"
	OrderStatusInProgress = "in_progress"
	OrderStatusCompleted  = "completed"
	OrderStatusCancelled  = "cancelled"
)
