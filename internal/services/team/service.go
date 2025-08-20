package team

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/models"
)

var (
	ErrTeamNotFound     = errors.New("team not found")
	ErrTeamNameExists   = errors.New("team name already exists")
	ErrUserNotInTeam    = errors.New("user not in team")
	ErrInsufficientRole = errors.New("insufficient role permissions")
	ErrBudgetExceeded   = errors.New("team budget exceeded")
)

type TeamService struct {
	db *gorm.DB
}

func NewTeamService(db *gorm.DB) *TeamService {
	return &TeamService{db: db}
}

type CreateTeamRequest struct {
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	MaxBudget        float64             `json:"max_budget"`
	BudgetDuration   models.BudgetPeriod `json:"budget_duration"`
	TPM              int                 `json:"tpm"`
	RPM              int                 `json:"rpm"`
	MaxParallelCalls int                 `json:"max_parallel_calls"`
	AllowedModels    []string            `json:"allowed_models"`
	BlockedModels    []string            `json:"blocked_models"`
}

type AddMemberRequest struct {
	UserID    uuid.UUID       `json:"user_id"`
	Role      models.TeamRole `json:"role"`
	MaxBudget *float64        `json:"max_budget,omitempty"`
	CustomTPM *int            `json:"custom_tpm,omitempty"`
	CustomRPM *int            `json:"custom_rpm,omitempty"`
}

// CreateTeam creates a new team
func (s *TeamService) CreateTeam(ctx context.Context, req *CreateTeamRequest, ownerID uuid.UUID) (*models.Team, error) {
	// Check if team name already exists
	var count int64
	if err := s.db.Model(&models.Team{}).Where("name = ?", req.Name).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrTeamNameExists
	}

	team := &models.Team{
		Name:             req.Name,
		Description:      req.Description,
		MaxBudget:        req.MaxBudget,
		BudgetDuration:   req.BudgetDuration,
		TPM:              req.TPM,
		RPM:              req.RPM,
		MaxParallelCalls: req.MaxParallelCalls,
		AllowedModels:    req.AllowedModels,
		BlockedModels:    req.BlockedModels,
		IsActive:         true,
	}

	// Set budget reset time
	now := time.Now()
	switch req.BudgetDuration {
	case models.BudgetPeriodDaily:
		team.BudgetResetAt = now.AddDate(0, 0, 1)
	case models.BudgetPeriodWeekly:
		team.BudgetResetAt = now.AddDate(0, 0, 7)
	case models.BudgetPeriodMonthly:
		team.BudgetResetAt = now.AddDate(0, 1, 0)
	case models.BudgetPeriodYearly:
		team.BudgetResetAt = now.AddDate(1, 0, 0)
	default:
		team.BudgetResetAt = now.AddDate(0, 0, 30)
	}

	// Create team and add owner as first member
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(team).Error; err != nil {
			return err
		}

		// Skip adding master key user as team member - master key has full access anyway
		if ownerID.String() == "00000000-0000-0000-0000-000000000001" {
			return nil
		}

		// Add owner as team member
		member := &models.TeamMember{
			TeamID:   team.ID,
			UserID:   ownerID,
			Role:     models.TeamRoleOwner,
			JoinedAt: now,
		}
		return tx.Create(member).Error
	})

	if err != nil {
		return nil, err
	}

	return team, nil
}

// GetTeam gets a team by ID
func (s *TeamService) GetTeam(ctx context.Context, teamID uuid.UUID) (*models.Team, error) {
	var team models.Team
	err := s.db.Preload("Members.User").First(&team, "id = ?", teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}
	return &team, nil
}

// UpdateTeam updates team settings
func (s *TeamService) UpdateTeam(ctx context.Context, teamID uuid.UUID, updates map[string]interface{}) (*models.Team, error) {
	var team models.Team
	if err := s.db.First(&team, "id = ?", teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}

	if err := s.db.Model(&team).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &team, nil
}

// DeleteTeam deletes a team
func (s *TeamService) DeleteTeam(ctx context.Context, teamID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete team members
		if err := tx.Delete(&models.TeamMember{}, "team_id = ?", teamID).Error; err != nil {
			return err
		}

		// Delete team
		result := tx.Delete(&models.Team{}, "id = ?", teamID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrTeamNotFound
		}

		return nil
	})
}

// AddMember adds a user to a team
func (s *TeamService) AddMember(ctx context.Context, teamID uuid.UUID, req *AddMemberRequest) (*models.TeamMember, error) {
	// Check if team exists
	var team models.Team
	if err := s.db.First(&team, "id = ?", teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}

	// Check if user is already a member
	var count int64
	if err := s.db.Model(&models.TeamMember{}).
		Where("team_id = ? AND user_id = ?", teamID, req.UserID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("user is already a team member")
	}

	member := &models.TeamMember{
		TeamID:    teamID,
		UserID:    req.UserID,
		Role:      req.Role,
		MaxBudget: req.MaxBudget,
		CustomTPM: req.CustomTPM,
		CustomRPM: req.CustomRPM,
		JoinedAt:  time.Now(),
	}

	if err := s.db.Create(member).Error; err != nil {
		return nil, err
	}

	// Load user details
	if err := s.db.Preload("User").First(member, "id = ?", member.ID).Error; err != nil {
		return nil, err
	}

	return member, nil
}

// UpdateMember updates a team member's settings
func (s *TeamService) UpdateMember(ctx context.Context, teamID, userID uuid.UUID, updates map[string]interface{}) (*models.TeamMember, error) {
	var member models.TeamMember
	err := s.db.Where("team_id = ? AND user_id = ?", teamID, userID).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotInTeam
		}
		return nil, err
	}

	if err := s.db.Model(&member).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &member, nil
}

// RemoveMember removes a user from a team
func (s *TeamService) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	result := s.db.Delete(&models.TeamMember{}, "team_id = ? AND user_id = ?", teamID, userID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotInTeam
	}
	return nil
}

// GetMemberRole gets a user's role in a team
func (s *TeamService) GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (models.TeamRole, error) {
	var member models.TeamMember
	err := s.db.Where("team_id = ? AND user_id = ?", teamID, userID).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrUserNotInTeam
		}
		return "", err
	}
	return member.Role, nil
}

// CanManageTeam checks if a user can manage a team
func (s *TeamService) CanManageTeam(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	role, err := s.GetMemberRole(ctx, teamID, userID)
	if err != nil {
		return false, err
	}
	return role == models.TeamRoleOwner || role == models.TeamRoleAdmin, nil
}

// ListTeams lists teams for a user
func (s *TeamService) ListTeams(ctx context.Context, userID *uuid.UUID, limit, offset int) ([]*models.Team, int64, error) {
	countQuery := s.db.Model(&models.Team{})
	dataQuery := s.db.Model(&models.Team{})

	if userID != nil {
		// Get teams where user is a member
		countQuery = countQuery.Joins("JOIN team_members ON team_members.team_id = teams.id").
			Where("team_members.user_id = ?", *userID)
		dataQuery = dataQuery.Joins("JOIN team_members ON team_members.team_id = teams.id").
			Where("team_members.user_id = ?", *userID)
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var teams []*models.Team
	err := dataQuery.Preload("Members").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&teams).Error
	if err != nil {
		return nil, 0, err
	}

	return teams, total, nil
}

// RecordUsage records usage for a team
func (s *TeamService) RecordUsage(ctx context.Context, teamID uuid.UUID, cost float64) error {
	var team models.Team
	if err := s.db.First(&team, "id = ?", teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTeamNotFound
		}
		return err
	}

	// Check and reset budget if needed
	if team.ShouldResetBudget() {
		team.ResetBudget()
	}

	team.CurrentSpend += cost

	// Check if budget exceeded
	if team.IsBudgetExceeded() {
		return ErrBudgetExceeded
	}

	return s.db.Save(&team).Error
}

// GetTeamStats gets usage statistics for a team
func (s *TeamService) GetTeamStats(ctx context.Context, teamID uuid.UUID) (map[string]interface{}, error) {
	var team models.Team
	if err := s.db.First(&team, "id = ?", teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}

	// Count members
	var memberCount int64
	s.db.Model(&models.TeamMember{}).Where("team_id = ?", teamID).Count(&memberCount)

	// Count keys
	var keyCount int64
	s.db.Model(&models.Key{}).Where("team_id = ?", teamID).Count(&keyCount)

	stats := map[string]interface{}{
		"team_id":           team.ID,
		"name":              team.Name,
		"member_count":      memberCount,
		"key_count":         keyCount,
		"current_spend":     team.CurrentSpend,
		"max_budget":        team.MaxBudget,
		"budget_remaining":  team.MaxBudget - team.CurrentSpend,
		"budget_percentage": 0.0,
		"budget_reset_at":   team.BudgetResetAt,
		"is_active":         team.IsActive,
		"created_at":        team.CreatedAt,
	}

	if team.MaxBudget > 0 {
		stats["budget_percentage"] = (team.CurrentSpend / team.MaxBudget) * 100
	}

	return stats, nil
}

// CheckBudgetAndRecord checks team budget and records usage
func (s *TeamService) CheckBudgetAndRecord(ctx context.Context, teamID uuid.UUID, cost float64) error {
	var team models.Team
	if err := s.db.First(&team, "id = ?", teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTeamNotFound
		}
		return err
	}

	// Check and reset budget if needed
	if team.ShouldResetBudget() {
		team.ResetBudget()
	}

	// Check if adding this cost would exceed budget
	if team.MaxBudget > 0 && (team.CurrentSpend+cost) > team.MaxBudget {
		return ErrBudgetExceeded
	}

	// Record the usage
	team.CurrentSpend += cost

	return s.db.Save(&team).Error
}

// IsTeamMember checks if a user is a member of a team
func (s *TeamService) IsTeamMember(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	_, err := s.GetMemberRole(ctx, teamID, userID)
	if err != nil {
		if err == ErrUserNotInTeam {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetTeamByName gets a team by name
func (s *TeamService) GetTeamByName(ctx context.Context, name string) (*models.Team, error) {
	var team models.Team
	err := s.db.Preload("Members.User").Where("name = ?", name).First(&team).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}
	return &team, nil
}

// GetTeamMembers gets all members of a team
func (s *TeamService) GetTeamMembers(ctx context.Context, teamID uuid.UUID) ([]*models.TeamMember, error) {
	var members []*models.TeamMember
	err := s.db.Preload("User").Where("team_id = ?", teamID).Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// GetOrCreateDefaultTeam gets or creates the default team for auto-provisioning
func (s *TeamService) GetOrCreateDefaultTeam(ctx context.Context) (*models.Team, error) {
	const defaultTeamName = "default"

	// Try to find existing default team
	team, err := s.GetTeamByName(ctx, defaultTeamName)
	if err == nil {
		return team, nil
	}
	if err != ErrTeamNotFound {
		return nil, err
	}

	// Create default team with reasonable defaults
	req := &CreateTeamRequest{
		Name:             defaultTeamName,
		Description:      "Default team for auto-provisioned users",
		MaxBudget:        100.0, // $100 per month
		BudgetDuration:   models.BudgetPeriodMonthly,
		TPM:              1000, // 1000 tokens per minute
		RPM:              100,  // 100 requests per minute
		MaxParallelCalls: 5,
		AllowedModels:    []string{"gpt-3.5-turbo", "gpt-4o-mini"}, // Safe, cost-effective models
		BlockedModels:    []string{},                               // No blocked models initially
	}

	// Use master key user ID as owner (won't be added as member due to special handling)
	masterKeyUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	return s.CreateTeam(ctx, req, masterKeyUserID)
}

// AddUserToDefaultTeam adds a user to the default team with appropriate role
func (s *TeamService) AddUserToDefaultTeam(ctx context.Context, userID uuid.UUID, role models.TeamRole) (*models.TeamMember, error) {
	// Get or create default team
	defaultTeam, err := s.GetOrCreateDefaultTeam(ctx)
	if err != nil {
		return nil, err
	}

	// Check if user is already a member
	var count int64
	if err := s.db.Model(&models.TeamMember{}).
		Where("team_id = ? AND user_id = ?", defaultTeam.ID, userID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		// User already in default team
		var member models.TeamMember
		err := s.db.Preload("User").Where("team_id = ? AND user_id = ?", defaultTeam.ID, userID).First(&member).Error
		return &member, err
	}

	// Set user budget based on role
	var maxBudget *float64
	switch role {
	case models.TeamRoleMember:
		budget := 10.0 // $10 per user
		maxBudget = &budget
	case models.TeamRoleAdmin:
		budget := 50.0 // $50 for admins
		maxBudget = &budget
	}

	req := &AddMemberRequest{
		UserID:    userID,
		Role:      role,
		MaxBudget: maxBudget,
		CustomTPM: nil, // Use team defaults
		CustomRPM: nil, // Use team defaults
	}

	return s.AddMember(ctx, defaultTeam.ID, req)
}
