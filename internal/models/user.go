package models

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	BaseModel
	Email           string     `gorm:"uniqueIndex;not null" json:"email"`
	Username        string     `gorm:"uniqueIndex;not null" json:"username"`
	Password        string     `gorm:"not null" json:"-"`
	FirstName       string     `json:"first_name"`
	LastName        string     `json:"last_name"`
	Role            UserRole   `gorm:"type:varchar(20);default:'user'" json:"role"`
	IsActive        bool       `gorm:"default:true" json:"is_active"`
	EmailVerified   bool       `gorm:"default:false" json:"email_verified"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	PasswordChangedAt *time.Time `json:"-"`
	
	// Relationships
	APIKeys []APIKey `gorm:"foreignKey:UserID" json:"-"`
	Groups  []Group  `gorm:"many2many:user_groups;" json:"groups,omitempty"`
	Budgets []Budget `gorm:"foreignKey:UserID" json:"-"`
	Usage   []Usage  `gorm:"foreignKey:UserID" json:"-"`
}

type UserRole string

const (
	RoleAdmin     UserRole = "admin"
	RoleManager   UserRole = "manager"
	RoleUser      UserRole = "user"
	RoleViewer    UserRole = "viewer"
)

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if err := u.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}
	
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), 12)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	
	now := time.Now()
	u.PasswordChangedAt = &now
	
	return nil
}

func (u *User) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement.Changed("Password") {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), 12)
		if err != nil {
			return err
		}
		u.Password = string(hashedPassword)
		
		now := time.Now()
		u.PasswordChangedAt = &now
	}
	return nil
}

func (u *User) ComparePassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

type UserGroup struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	GroupID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Role      GroupRole `gorm:"type:varchar(20);default:'member'"`
	JoinedAt  time.Time
	ExpiresAt *time.Time
}

type GroupRole string

const (
	GroupRoleOwner   GroupRole = "owner"
	GroupRoleAdmin   GroupRole = "admin"
	GroupRoleMember  GroupRole = "member"
	GroupRoleViewer  GroupRole = "viewer"
)