package cel

import (
	"time"

	"github.com/amerfu/pllm/internal/core/models"
)

// KeyToInput converts a Key row into the CEL input map for the
// `keys` scan target. Pointer-typed timestamps and UUIDs are surfaced
// as nil so expressions can use `key.last_used_at != null` to gate
// time-based predicates without panicking on never-used keys.
func KeyToInput(k *models.Key) map[string]any {
	in := map[string]any{
		"id":          k.ID.String(),
		"name":        k.Name,
		"key_prefix":  k.KeyPrefix,
		"type":        string(k.Type),
		"is_active":   k.IsActive,
		"created_at":  k.CreatedAt,
		"tags":        toStringSlice(k.Tags),
		"usage_count": k.UsageCount,
	}
	in["last_used_at"] = nullableTime(k.LastUsedAt)
	in["expires_at"] = nullableTime(k.ExpiresAt)
	in["revoked_at"] = nullableTime(k.RevokedAt)
	if k.UserID != nil {
		in["user_id"] = k.UserID.String()
	} else {
		in["user_id"] = nil
	}
	if k.TeamID != nil {
		in["team_id"] = k.TeamID.String()
	} else {
		in["team_id"] = nil
	}
	return in
}

// ServerToInput is the same idea for RegistryServer rows.
func ServerToInput(s *models.RegistryServer) map[string]any {
	return map[string]any{
		"id":            s.ID.String(),
		"name":          s.Name,
		"version":       s.Version,
		"title":         s.Title,
		"status":        string(s.Status),
		"is_latest":     s.IsLatest,
		"has_publisher": s.PublishedByUserID != nil,
		"created_at":    s.CreatedAt,
		"published_at":  s.PublishedAt,
	}
}

// UseCaseToInput maps a UseCase row.
func UseCaseToInput(u *models.UseCase) map[string]any {
	return map[string]any{
		"id":                     u.ID.String(),
		"name":                   u.Name,
		"slug":                   u.Slug,
		"status":                 string(u.Status),
		"risk_tier":              string(u.RiskTier),
		"enforce_scope":          u.EnforceScope,
		"compliance_flag":        u.ComplianceFlag,
		"created_at":             u.CreatedAt,
		"updated_at":             u.UpdatedAt,
		"last_graph_computed_at": nullableTime(u.LastGraphComputedAt),
	}
}

// UserToInput maps a User row.
func UserToInput(u *models.User) map[string]any {
	return map[string]any{
		"id":                u.ID.String(),
		"email":             u.Email,
		"username":          u.Username,
		"role":              string(u.Role),
		"is_active":         u.IsActive,
		"email_verified":    u.EmailVerified,
		"created_at":        u.CreatedAt,
		"last_login_at":     nullableTime(u.LastLoginAt),
		"external_provider": u.ExternalProvider,
	}
}

// nullableTime surfaces a nil *time.Time as Go nil, which CEL maps
// to its `null` value. A non-nil pointer is dereferenced so the
// timestamp arrives as time.Time and the timestamp arithmetic
// operators (`now - t`, `duration(...)`, comparison) all work.
func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

// toStringSlice copies a pq.StringArray (or any []string-aliased type)
// into a fresh []string the cel-go map adapter accepts without
// reflection surprises.
func toStringSlice(s []string) []string {
	if len(s) == 0 {
		return []string{}
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}
