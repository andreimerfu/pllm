# pLLM Authentication & User Management Implementation Plan

## Executive Summary

This document provides a comprehensive analysis and implementation plan for enhancing pLLM's authentication and user management system, comparing it with LiteLLM's approach and incorporating best practices from both systems while leveraging Dex OAuth2 for enterprise-grade authentication.

## Table of Contents

1. [Current State Analysis](#current-state-analysis)
2. [LiteLLM Comparison](#litellm-comparison)
3. [Implementation Gaps](#implementation-gaps)
4. [Proposed Architecture](#proposed-architecture)
5. [Implementation Roadmap](#implementation-roadmap)
6. [Configuration Examples](#configuration-examples)
7. [Success Metrics](#success-metrics)

## Current State Analysis

### pLLM Current Implementation

#### Backend (Go)
- **User Model**: Comprehensive with roles (admin, manager, user, viewer), budget control, rate limiting
- **Team Model**: Hierarchical structure with member roles and shared resources
- **API Keys**: Hash-based security with scopes and usage tracking
- **Virtual Keys**: Flexible ownership model (user or team) with granular controls
- **Authentication**: Dex OAuth2 integration with PKCE support
- **Database**: PostgreSQL via GORM ORM

#### Frontend (React)
- **Dual Auth Contexts**: Basic AuthContext and OIDCAuthContext
- **PKCE Implementation**: Secure OAuth2 flow
- **Session Management**: Token storage with silent renewal

### LiteLLM Implementation

- **Master Key Concept**: Bootstrap admin access via config
- **Virtual Keys**: Standardized `sk-` prefix format
- **Auto-provisioning**: Optional user creation on first login
- **CLI Tools**: Comprehensive management commands
- **Spend Tracking**: Detailed analytics at key/user/team levels
- **Budget Alerts**: Configurable threshold notifications

## LiteLLM Comparison

| Feature | pLLM (Current) | LiteLLM | Recommended Approach |
|---------|----------------|---------|---------------------|
| **Authentication** | Dex OAuth2 + API Keys | Master Key + Virtual Keys | Hybrid: Dex + Master Key + Virtual Keys |
| **User Management** | Manual creation | Auto-provisioning optional | Auto-provision from Dex with role mapping |
| **Team Structure** | Hierarchical with roles | Basic teams | Enhanced hierarchical with inheritance |
| **Key Types** | API Keys + Virtual Keys | Virtual Keys only | Unified key system with types |
| **Budget Control** | User/Team/Key levels | Similar | Enhanced with cascade and alerts |
| **CLI Tools** | None | Comprehensive | Add CLI for all operations |
| **Spend Tracking** | Basic | Detailed | Enhanced analytics and reporting |
| **UI Auth** | Full OIDC | Basic auth | Keep OIDC with master key fallback |

## Implementation Gaps

### Critical Priority
1. **Master Admin Key System**
   - No bootstrap admin access without Dex
   - Solution: Config-based master key for initial setup

2. **User Auto-provisioning**
   - Manual user creation after OAuth login
   - Solution: Automatic user creation with Dex claims mapping

### High Priority
3. **Spend Tracking & Analytics**
   - Basic usage tracking without detailed reporting
   - Solution: Enhanced analytics with cost breakdowns

4. **CLI Management Tools**
   - No command-line administration
   - Solution: Cobra-based CLI for all operations

5. **Budget Alert System**
   - No proactive notifications
   - Solution: Webhook/email alerts at thresholds

## Proposed Architecture

### Authentication Layers

```mermaid
graph TD
    A[Master Key] --> B[System Admin]
    C[Dex OAuth2] --> D[User Authentication]
    D --> E[Auto-provisioning]
    E --> F[Team Assignment]
    G[API Keys] --> H[Programmatic Access]
    I[Virtual Keys] --> J[Scoped Access]
```

### User Journey Flow

#### Initial Setup
```
1. Deploy with master key in config
2. Master key creates default admin
3. Admin configures Dex integration
4. Users login via Dex
5. Auto-provisioning creates user records
6. Admin assigns users to teams
7. Users generate API keys for programmatic access
```

#### Operational Flow
```
Dex Login → User Creation → Team Assignment → Budget Allocation → Key Generation → API Usage → Usage Tracking → Budget Alerts
```

### Permission Hierarchy

1. **System Admin** (Master Key)
   - Full system control
   - User/team management
   - Configuration changes

2. **Organization Admin** (Dex admin group)
   - User management
   - Team creation
   - Budget allocation

3. **Team Owner**
   - Team member management
   - Team key generation
   - Team budget control

4. **Team Member**
   - Personal key generation
   - API usage
   - View team resources

5. **Guest** (Virtual Key)
   - Limited, scoped access
   - Temporary usage
   - Specific model access

## Implementation Roadmap

### Phase 1: Foundation (Weeks 1-2)

#### 1.1 Master Key System

```go
// config/config.go
type GeneralSettings struct {
    MasterKey         string `yaml:"master_key" env:"PLLM_MASTER_KEY"`
    DefaultAdminEmail string `yaml:"default_admin_email"`
    EnableAutoUsers   bool   `yaml:"enable_auto_user_creation"`
}
```

#### 1.2 Database Schema Updates

```sql
-- Users table additions
ALTER TABLE users ADD COLUMN 
    created_via VARCHAR(20) DEFAULT 'manual',
    dex_subject VARCHAR(255),
    last_sync_at TIMESTAMP;

-- Master keys table
CREATE TABLE master_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    last_used_at TIMESTAMP,
    is_active BOOLEAN DEFAULT true
);

-- Audit logs table
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id UUID,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### Phase 2: Dex Integration Enhancement (Weeks 2-3)

#### 2.1 Auto User Provisioning

```go
// auth/dex_handler.go
func HandleDexCallback(claims *AuthClaims) (*User, error) {
    user := FindUserByEmail(claims.Email)
    
    if user == nil && config.EnableAutoUserCreation {
        user = &User{
            Email:      claims.Email,
            Username:   claims.PreferredUsername,
            Role:       DetermineRoleFromGroups(claims.Groups),
            CreatedVia: "dex",
            DexSubject: claims.Subject,
        }
        CreateUser(user)
        AssignToTeamsFromGroups(user, claims.Groups)
    }
    
    return user, nil
}
```

#### 2.2 Group Mapping Configuration

```yaml
# config.yaml
dex_mapping:
  group_to_team:
    "engineering": "Engineering Team"
    "sales": "Sales Team"
  group_to_role:
    "admin": "admin"
    "developer": "user"
  auto_create_teams: true
```

### Phase 3: Unified Key Management (Weeks 3-4)

#### 3.1 Key Type System

```go
type KeyType string

const (
    KeyTypeMaster  KeyType = "master"
    KeyTypeAdmin   KeyType = "admin"
    KeyTypeTeam    KeyType = "team"
    KeyTypeUser    KeyType = "user"
    KeyTypeVirtual KeyType = "virtual"
)

type UnifiedKey struct {
    BaseModel
    Key       string    `gorm:"uniqueIndex"`
    KeyType   KeyType
    Name      string
    OwnerID   *uuid.UUID
    OwnerType string
    
    // Permissions
    Scopes         []string
    CanManageUsers bool
    CanManageTeams bool
    
    // Limits
    MaxBudget *float64
    TPM       *int
    RPM       *int
    
    // Tracking
    CurrentSpend float64
    LastUsedAt   *time.Time
    ExpiresAt    *time.Time
}
```

### Phase 4: Budget & Analytics (Weeks 4-5)

#### 4.1 Usage Tracking Service

```go
type UsageTracker struct {
    db *gorm.DB
}

func (ut *UsageTracker) TrackUsage(req UsageRequest) {
    ut.trackKeyUsage(req)
    ut.trackUserUsage(req)
    ut.trackTeamUsage(req)
    ut.trackModelUsage(req)
    ut.checkBudgetLimits(req)
    ut.sendBudgetAlerts(req)
}
```

#### 4.2 Alert System

```go
type BudgetAlertService struct {
    webhookClient *WebhookClient
    emailClient   *EmailClient
}

func (bas *BudgetAlertService) CheckAndAlert(entity BudgetEntity) {
    percentage := (entity.CurrentSpend / entity.MaxBudget) * 100
    
    for _, threshold := range []float64{50, 75, 90, 100} {
        if percentage >= threshold && !entity.AlertedAt(threshold) {
            bas.SendAlert(entity, threshold)
            entity.MarkAlerted(threshold)
        }
    }
}
```

### Phase 5: Frontend Enhancement (Weeks 5-6)

#### 5.1 Admin Dashboard

```tsx
export function AdminDashboard() {
    return (
        <>
            <UserManagement />
            <TeamManagement />
            <KeyManagement />
            <UsageAnalytics />
            <BudgetMonitoring />
        </>
    );
}
```

#### 5.2 Self-Service Portal

```tsx
export function UserPortal() {
    return (
        <>
            <MyKeys />
            <UsageStats />
            <TeamMemberships />
            <BudgetStatus />
        </>
    );
}
```

### Phase 6: CLI Tools (Week 6)

```bash
# User management
pllm user create --email user@example.com --role admin
pllm user list --team engineering
pllm user assign-team --user-id xxx --team-id yyy

# Key management
pllm key generate --type team --team-id xxx
pllm key revoke --key-id xxx
pllm key list --user-id xxx

# Team management
pllm team create --name "Engineering"
pllm team set-budget --team-id xxx --amount 1000
```

## Configuration Examples

### Complete Configuration

```yaml
general_settings:
  master_key: "${PLLM_MASTER_KEY:pllm_sk_master_}"
  database_url: "postgresql://user:pass@localhost/pllm"
  enable_auto_user_creation: true
  default_admin_email: "admin@example.com"

auth:
  dex:
    enabled: true
    issuer: "http://dex:5556/dex"
    client_id: "pllm"
    client_secret: "${DEX_CLIENT_SECRET}"
    redirect_url: "http://localhost:8080/callback"
    auto_create_users: true
    default_role: "user"
    admin_groups: ["admin", "platform-admin"]

dex_mapping:
  group_to_team:
    "engineering": "Engineering Team"
    "sales": "Sales Team"
    "support": "Support Team"
  group_to_role:
    "admin": "admin"
    "manager": "manager"
    "developer": "user"
    "viewer": "viewer"
  auto_create_teams: true

budget:
  enable_alerts: true
  alert_webhook: "${BUDGET_ALERT_WEBHOOK}"
  alert_email: "budget-alerts@example.com"
  alert_thresholds: [50, 75, 90, 100]
  
rate_limits:
  default_tpm: 100000
  default_rpm: 100
  default_parallel: 10
  
models:
  default_allowed: ["gpt-4", "gpt-3.5-turbo", "claude-3"]
  default_blocked: []
```

### Docker Compose Integration

```yaml
version: '3.8'
services:
  pllm:
    image: pllm:latest
    environment:
      PLLM_MASTER_KEY: ${PLLM_MASTER_KEY}
      DEX_CLIENT_SECRET: ${DEX_CLIENT_SECRET}
      DATABASE_URL: postgresql://pllm:password@postgres/pllm
    depends_on:
      - postgres
      - dex
  
  dex:
    image: dexidp/dex:latest
    volumes:
      - ./dex-config.yaml:/etc/dex/config.yaml
    ports:
      - "5556:5556"
  
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: pllm
      POSTGRES_USER: pllm
      POSTGRES_PASSWORD: password
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
```

## Success Metrics

### Technical Metrics
- **Authentication Latency**: < 100ms
- **Key Generation Time**: < 50ms
- **Budget Check Overhead**: < 10ms
- **System Uptime**: 99.9%

### Business Metrics
- **User Provisioning Time**: < 1 minute (from first login)
- **Budget Visibility**: 100% of usage tracked
- **Support Tickets**: 50% reduction in auth-related issues
- **Admin Efficiency**: 75% reduction in manual operations

### Security Metrics
- **Authentication Coverage**: 100% of API calls
- **Audit Trail**: Complete for all admin actions
- **Key Rotation**: Monthly for high-privilege keys
- **Vulnerability Response**: < 24 hours for critical issues

## Risk Mitigation

### Migration Strategy
1. **Backward Compatibility**: All changes maintain existing API contracts
2. **Staged Rollout**: Feature flags for gradual enablement
3. **Rollback Plan**: Database migrations reversible
4. **Testing**: Comprehensive test coverage before production

### Security Considerations
1. **Key Storage**: All keys hashed with bcrypt
2. **Audit Logging**: Every administrative action logged
3. **Rate Limiting**: Prevent abuse at multiple levels
4. **Budget Controls**: Hard limits with automatic enforcement

### Performance Optimization
1. **Caching**: Redis for session and rate limit data
2. **Database Indexes**: Optimized for common queries
3. **Connection Pooling**: Efficient database connections
4. **Async Processing**: Background jobs for analytics

## Conclusion

This implementation plan provides a comprehensive roadmap for enhancing pLLM's authentication and user management system. By combining the best features from LiteLLM with pLLM's superior Dex integration, we create an enterprise-ready solution that offers:

- **Seamless User Experience**: Auto-provisioning with SSO
- **Flexible Access Control**: Multiple key types with granular permissions
- **Comprehensive Monitoring**: Detailed usage tracking and alerts
- **Operational Excellence**: CLI tools and admin dashboard
- **Enterprise Security**: Multi-layer authentication with audit trails

The phased approach ensures minimal disruption while progressively adding value, with clear success metrics to measure progress and impact.