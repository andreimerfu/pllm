package models

import (
	"github.com/google/uuid"
)

// Route represents a virtual endpoint that distributes traffic across multiple models.
// Clients use the route slug as the model name (e.g., model: "smart") and the gateway
// picks the best model based on the route's strategy.
type Route struct {
	BaseModel
	Name           string          `gorm:"not null" json:"name"`
	Slug           string          `gorm:"uniqueIndex;not null" json:"slug"`
	Description    string          `json:"description,omitempty"`
	Strategy       string          `gorm:"not null;default:'priority'" json:"strategy"`
	FallbackModels StringArrayJSON `gorm:"type:jsonb" json:"fallback_models,omitempty"`
	Enabled        bool            `gorm:"default:true" json:"enabled"`
	Source         string          `gorm:"default:'user'" json:"source"`
	CreatedByID    *uuid.UUID      `gorm:"type:uuid" json:"created_by_id,omitempty"`
	Models         []RouteModel    `gorm:"foreignKey:RouteID" json:"models,omitempty"`
}

// TableName overrides the default table name.
func (Route) TableName() string {
	return "routes"
}

// RouteModel represents a model entry within a route, with weight and priority for
// the route-level routing strategy.
type RouteModel struct {
	BaseModel
	RouteID   uuid.UUID `gorm:"type:uuid;not null;index" json:"route_id"`
	ModelName string    `gorm:"not null" json:"model_name"`
	Weight    int       `gorm:"default:50" json:"weight"`
	Priority  int       `gorm:"default:50" json:"priority"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
}

// TableName overrides the default table name.
func (RouteModel) TableName() string {
	return "route_models"
}
