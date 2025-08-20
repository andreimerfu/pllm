package auth

import (
	"context"
	"sync"

	"github.com/amerfu/pllm/internal/models"
	"github.com/google/uuid"
)

// Permission represents an action that can be performed
type Permission string

// Global permissions
const (
	// User management
	PermUsersCreate Permission = "users:create"
	PermUsersRead   Permission = "users:read"
	PermUsersUpdate Permission = "users:update"
	PermUsersDelete Permission = "users:delete"

	// Team management
	PermTeamsCreate        Permission = "teams:create"
	PermTeamsRead          Permission = "teams:read"
	PermTeamsUpdate        Permission = "teams:update"
	PermTeamsDelete        Permission = "teams:delete"
	PermTeamsManageMembers Permission = "teams:manage_members"

	// Key management
	PermKeysCreate Permission = "keys:create"
	PermKeysRead   Permission = "keys:read"
	PermKeysUpdate Permission = "keys:update"
	PermKeysRevoke Permission = "keys:revoke"

	// Budget management
	PermBudgetsCreate Permission = "budgets:create"
	PermBudgetsRead   Permission = "budgets:read"
	PermBudgetsUpdate Permission = "budgets:update"
	PermBudgetsDelete Permission = "budgets:delete"

	// Analytics
	PermAnalyticsRead   Permission = "analytics:read"
	PermAnalyticsExport Permission = "analytics:export"

	// Models
	PermModelsRead Permission = "models:read"
	PermModelsUse  Permission = "models:use"

	// Dashboard
	PermDashboardRead Permission = "dashboard:read"

	// Chat
	PermChatUse Permission = "chat:use"

	// System
	PermSystemConfig Permission = "system:config"
	PermSystemAudit  Permission = "system:audit"

	// Special permissions
	PermAll Permission = "*" // Super admin permission
)

// PermissionService manages permissions and authorization
type PermissionService struct {
	mu              sync.RWMutex
	rolePermissions map[models.UserRole][]Permission
	teamPermissions map[models.TeamRole][]Permission
}

// NewPermissionService creates a new permission service
func NewPermissionService() *PermissionService {
	ps := &PermissionService{
		rolePermissions: make(map[models.UserRole][]Permission),
		teamPermissions: make(map[models.TeamRole][]Permission),
	}
	ps.initializePermissions()
	return ps
}

// initializePermissions sets up the default role-permission mappings
func (ps *PermissionService) initializePermissions() {
	// Global role permissions
	ps.rolePermissions[models.RoleAdmin] = []Permission{
		PermAll, // Admins have all permissions
	}

	ps.rolePermissions[models.RoleManager] = []Permission{
		PermUsersRead, PermUsersUpdate,
		PermTeamsCreate, PermTeamsRead, PermTeamsUpdate, PermTeamsManageMembers,
		PermKeysCreate, PermKeysRead, PermKeysUpdate, PermKeysRevoke,
		PermBudgetsCreate, PermBudgetsRead, PermBudgetsUpdate,
		PermAnalyticsRead, PermAnalyticsExport,
	}

	ps.rolePermissions[models.RoleUser] = []Permission{
		PermUsersRead,                                // Can read their own user info
		PermTeamsRead,                                // Can read teams they belong to
		PermKeysCreate, PermKeysRead, PermKeysUpdate, // Can manage their own keys
		PermBudgetsRead,               // Can view budgets
		PermAnalyticsRead,             // Can view their own analytics
		PermModelsRead, PermModelsUse, // Can view and use models
		PermDashboardRead, // Can access dashboard
		PermChatUse,       // Can use chat functionality
	}

	ps.rolePermissions[models.RoleViewer] = []Permission{
		PermUsersRead,
		PermTeamsRead,
		PermKeysRead,
		PermBudgetsRead,
		PermAnalyticsRead,
		PermModelsRead,    // Can view models
		PermDashboardRead, // Can access dashboard
	}

	// Team role permissions (within team context)
	ps.teamPermissions[models.TeamRoleOwner] = []Permission{
		PermTeamsUpdate, PermTeamsDelete, PermTeamsManageMembers,
		PermKeysCreate, PermKeysRead, PermKeysUpdate, PermKeysRevoke,
		PermBudgetsCreate, PermBudgetsRead, PermBudgetsUpdate, PermBudgetsDelete,
		PermAnalyticsRead, PermAnalyticsExport,
	}

	ps.teamPermissions[models.TeamRoleAdmin] = []Permission{
		PermTeamsUpdate, PermTeamsManageMembers,
		PermKeysCreate, PermKeysRead, PermKeysUpdate, PermKeysRevoke,
		PermBudgetsRead, PermBudgetsUpdate,
		PermAnalyticsRead, PermAnalyticsExport,
	}

	ps.teamPermissions[models.TeamRoleMember] = []Permission{
		PermTeamsRead,
		PermKeysCreate, PermKeysRead, PermKeysUpdate, // Can manage their own team keys
		PermBudgetsRead,
		PermAnalyticsRead,
	}

	ps.teamPermissions[models.TeamRoleViewer] = []Permission{
		PermTeamsRead,
		PermKeysRead,
		PermBudgetsRead,
		PermAnalyticsRead,
	}
}

// HasPermission checks if a user has a specific permission
func (ps *PermissionService) HasPermission(user *models.User, permission Permission) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Check global role permissions
	if perms, ok := ps.rolePermissions[user.Role]; ok {
		for _, p := range perms {
			if p == permission || p == PermAll {
				return true
			}
		}
	}

	return false
}

// HasTeamPermission checks if a user has a permission within a team context
func (ps *PermissionService) HasTeamPermission(user *models.User, teamID uuid.UUID, permission Permission) bool {
	// First check global permissions
	if ps.HasPermission(user, permission) {
		return true
	}

	// Then check team-specific permissions
	for _, membership := range user.Teams {
		if membership.TeamID == teamID {
			ps.mu.RLock()
			perms, ok := ps.teamPermissions[membership.Role]
			ps.mu.RUnlock()

			if ok {
				for _, p := range perms {
					if p == permission {
						return true
					}
				}
			}
		}
	}

	return false
}

// GetUserPermissions returns all permissions for a user
func (ps *PermissionService) GetUserPermissions(user *models.User) []Permission {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	permSet := make(map[Permission]bool)

	// Add global role permissions
	if perms, ok := ps.rolePermissions[user.Role]; ok {
		for _, p := range perms {
			permSet[p] = true
		}
	}

	// Add team permissions
	for _, membership := range user.Teams {
		if perms, ok := ps.teamPermissions[membership.Role]; ok {
			for _, p := range perms {
				permSet[p] = true
			}
		}
	}

	// Convert to slice
	var permissions []Permission
	for p := range permSet {
		permissions = append(permissions, p)
	}

	return permissions
}

// CanManageUser checks if user1 can manage user2
func (ps *PermissionService) CanManageUser(user1, user2 *models.User) bool {
	// Admins can manage anyone
	if ps.HasPermission(user1, PermUsersUpdate) {
		return true
	}

	// Managers can manage users and viewers
	if user1.Role == models.RoleManager &&
		(user2.Role == models.RoleUser || user2.Role == models.RoleViewer) {
		return true
	}

	// Users can only manage themselves
	if user1.ID == user2.ID {
		return true
	}

	return false
}

// CanManageTeam checks if a user can manage a team
func (ps *PermissionService) CanManageTeam(user *models.User, teamID uuid.UUID) bool {
	// Admins can manage any team
	if ps.HasPermission(user, PermTeamsUpdate) {
		return true
	}

	// Check if user is team owner or admin
	for _, membership := range user.Teams {
		if membership.TeamID == teamID {
			return membership.Role == models.TeamRoleOwner ||
				membership.Role == models.TeamRoleAdmin
		}
	}

	return false
}

// CanManageKey checks if a user can manage a key
func (ps *PermissionService) CanManageKey(user *models.User, key *models.Key) bool {
	// Admins can manage any key
	if ps.HasPermission(user, PermKeysUpdate) {
		return true
	}

	// Users can manage their own keys
	if key.UserID != nil && *key.UserID == user.ID {
		return true
	}

	// Team members can manage team keys based on their role
	if key.TeamID != nil {
		return ps.HasTeamPermission(user, *key.TeamID, PermKeysUpdate)
	}

	return false
}

// Context helpers

type contextKey string

const (
	permissionServiceKey contextKey = "permissionService"
	userPermissionsKey   contextKey = "userPermissions"
)

// WithPermissionService adds the permission service to context
func WithPermissionService(ctx context.Context, ps *PermissionService) context.Context {
	return context.WithValue(ctx, permissionServiceKey, ps)
}

// GetPermissionService gets the permission service from context
func GetPermissionService(ctx context.Context) *PermissionService {
	if ps, ok := ctx.Value(permissionServiceKey).(*PermissionService); ok {
		return ps
	}
	return nil
}

// WithUserPermissions adds user permissions to context
func WithUserPermissions(ctx context.Context, perms []Permission) context.Context {
	return context.WithValue(ctx, userPermissionsKey, perms)
}

// GetUserPermissions gets user permissions from context
func GetUserPermissions(ctx context.Context) []Permission {
	if perms, ok := ctx.Value(userPermissionsKey).([]Permission); ok {
		return perms
	}
	return nil
}
