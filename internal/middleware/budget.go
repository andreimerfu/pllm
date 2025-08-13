package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	
	"github.com/amerfu/pllm/internal/services/budget"
	"github.com/amerfu/pllm/internal/services/team"
	"github.com/amerfu/pllm/internal/services/virtualkey"
)

type BudgetMiddleware struct {
	logger        *zap.Logger
	budgetService *budget.BudgetService
	teamService   *team.TeamService
	keyService    *virtualkey.VirtualKeyService
}

type BudgetConfig struct {
	Logger        *zap.Logger
	BudgetService *budget.BudgetService
	TeamService   *team.TeamService
	KeyService    *virtualkey.VirtualKeyService
}

func NewBudgetMiddleware(config *BudgetConfig) *BudgetMiddleware {
	return &BudgetMiddleware{
		logger:        config.Logger,
		budgetService: config.BudgetService,
		teamService:   config.TeamService,
		keyService:    config.KeyService,
	}
}

// EnforceBudget checks budget limits before allowing requests
func (m *BudgetMiddleware) EnforceBudget(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip budget enforcement for health checks and non-API endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		// Get authentication type
		authType := GetAuthType(r.Context())
		
		// Skip for master key
		if authType == AuthTypeMasterKey {
			next.ServeHTTP(w, r)
			return
		}

		// Check virtual key budget
		if authType == AuthTypeVirtualKey {
			key, ok := GetVirtualKey(r.Context())
			if !ok {
				m.sendError(w, http.StatusInternalServerError, "Virtual key not found in context")
				return
			}

			// Check key-level budget
			if key.IsBudgetExceeded() {
				m.sendError(w, http.StatusPaymentRequired, "Key budget exceeded")
				return
			}

			// Check team budget if key belongs to a team
			if key.TeamID != nil {
				teamBudgetOK, err := m.checkTeamBudget(*key.TeamID)
				if err != nil {
					m.logger.Error("Failed to check team budget", zap.Error(err))
					// Continue on error to avoid blocking legitimate requests
				} else if !teamBudgetOK {
					m.sendError(w, http.StatusPaymentRequired, "Team budget exceeded")
					return
				}
			}

			// Check user budget if key belongs to a user
			if key.UserID != nil {
				userBudgetOK, err := m.checkUserBudget(*key.UserID)
				if err != nil {
					m.logger.Error("Failed to check user budget", zap.Error(err))
					// Continue on error
				} else if !userBudgetOK {
					m.sendError(w, http.StatusPaymentRequired, "User budget exceeded")
					return
				}
			}
		}

		// Check JWT user budget
		if authType == AuthTypeJWT {
			userID, ok := GetUserID(r.Context())
			if ok {
				userBudgetOK, err := m.checkUserBudget(userID)
				if err != nil {
					m.logger.Error("Failed to check user budget", zap.Error(err))
				} else if !userBudgetOK {
					m.sendError(w, http.StatusPaymentRequired, "User budget exceeded")
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// ValidateTeamAccess ensures the user has access to team resources
func (m *BudgetMiddleware) ValidateTeamAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authType := GetAuthType(r.Context())
		
		// Master key always has access
		if authType == AuthTypeMasterKey {
			next.ServeHTTP(w, r)
			return
		}

		// Extract team ID from context or URL
		teamID, hasTeamID := GetTeamID(r.Context())
		
		// If no team context, continue
		if !hasTeamID {
			next.ServeHTTP(w, r)
			return
		}

		// Check virtual key team access
		if authType == AuthTypeVirtualKey {
			key, ok := GetVirtualKey(r.Context())
			if !ok {
				m.sendError(w, http.StatusForbidden, "Team access denied")
				return
			}
			
			// Key must belong to the team or user must be team member
			if key.TeamID != nil && *key.TeamID == teamID {
				next.ServeHTTP(w, r)
				return
			}
			
			// Check if key's user is a team member
			if key.UserID != nil {
				isMember, err := m.isTeamMember(teamID, *key.UserID)
				if err != nil {
					m.logger.Error("Failed to check team membership", zap.Error(err))
					m.sendError(w, http.StatusInternalServerError, "Failed to validate team access")
					return
				}
				if !isMember {
					m.sendError(w, http.StatusForbidden, "Team access denied")
					return
				}
			}
		}

		// Check JWT user team membership
		if authType == AuthTypeJWT {
			userID, ok := GetUserID(r.Context())
			if !ok {
				m.sendError(w, http.StatusForbidden, "Team access denied")
				return
			}
			
			isMember, err := m.isTeamMember(teamID, userID)
			if err != nil {
				m.logger.Error("Failed to check team membership", zap.Error(err))
				m.sendError(w, http.StatusInternalServerError, "Failed to validate team access")
				return
			}
			if !isMember {
				m.sendError(w, http.StatusForbidden, "Team access denied")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (m *BudgetMiddleware) checkTeamBudget(teamID uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	team, err := m.teamService.GetTeam(ctx, teamID)
	if err != nil {
		return false, err
	}
	
	// Check if budget should be reset
	if team.ShouldResetBudget() {
		team.ResetBudget()
		_, err = m.teamService.UpdateTeam(ctx, teamID, map[string]interface{}{
			"current_spend":   team.CurrentSpend,
			"budget_reset_at": team.BudgetResetAt,
		})
		if err != nil {
			return false, err
		}
	}
	
	return !team.IsBudgetExceeded(), nil
}

func (m *BudgetMiddleware) checkUserBudget(userID uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Get active budgets for user
	budgets, err := m.budgetService.ListBudgets(ctx, &userID, nil)
	if err != nil {
		return false, err
	}
	
	for _, budget := range budgets {
		if budget.IsExceeded() {
			return false, nil
		}
	}
	
	return true, nil
}

func (m *BudgetMiddleware) isTeamMember(teamID, userID uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err := m.teamService.GetMemberRole(ctx, teamID, userID)
	if err != nil {
		if err == team.ErrUserNotInTeam {
			return false, nil
		}
		return false, err
	}
	
	return true, nil
}

func (m *BudgetMiddleware) sendError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	errorType := "budget_error"
	if statusCode == http.StatusForbidden {
		errorType = "access_error"
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    errorType,
			"code":    statusCode,
		},
	})
}