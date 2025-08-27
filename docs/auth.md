# Authentication & Authorization

PLLM provides enterprise-grade authentication with team-based access control, budget management, and multiple authentication methods.

## Authentication Methods

### 1. Master Key (Development)

For development and initial setup:

```bash
# Set in environment or config
PLLM_MASTER_KEY=sk-master-dev-key-change-in-production

# Use in API requests
curl -H "Authorization: Bearer sk-master-dev-key-change-in-production" \
  http://localhost:8080/v1/chat/completions
```

### 2. OIDC/OAuth2 via Dex

For production environments, PLLM integrates with **any identity provider supported by Dex**:

- **LDAP/Active Directory**
- **SAML 2.0 providers**
- **GitHub/GitLab**
- **Google Workspace** 
- **Microsoft Azure AD/Entra ID**
- **Keycloak**
- **Auth0**
- **Generic OpenID Connect**
- **Generic OAuth2**

Configuration in `config.yaml`:

```yaml
auth:
  require_auth: true
  dex:
    enabled: true
    issuer: "http://localhost:5556/dex"
    client_id: "pllm-web"
    client_secret: "pllm-web-secret"
    redirect_url: "http://localhost:3000/auth/callback"
    scopes: ["openid", "profile", "email", "groups"]
```

### 3. JWT Tokens

After OIDC authentication, users receive JWT tokens:

```yaml
jwt:
  secret_key: your-super-secret-jwt-key-change-this
  access_token_duration: 15m
  refresh_token_duration: 168h  # 7 days
```

## Team-Based Access Control

### Teams & Memberships

PLLM implements team-based organization:

- Users belong to teams
- Teams have budget limits and model access
- Team admins can manage members and API keys
- Role-based permissions within teams

### API Key Management

Create and manage API keys via:

**Admin UI** (`/ui`):
- Web interface for key management
- Team-based key creation
- Usage monitoring

**API Endpoints** (from `/internal/router/router.go`):
```bash
# List user's API keys
GET /v1/user/keys

# Create new API key  
POST /v1/user/keys

# Delete API key
DELETE /v1/user/keys/{key_id}
```

## Budget & Usage Tracking

### Asynchronous Budget System

PLLM uses a high-performance async budget system with Redis:

```go
// From router.go - Redis-based async budget middleware
asyncBudgetMiddleware := middleware.NewAsyncBudgetMiddleware(&middleware.AsyncBudgetConfig{
    BudgetCache: redisService.NewBudgetCache(redisClient, logger, 5*time.Minute),
    UsageQueue:  redisService.NewUsageQueue(&redisService.UsageQueueConfig{
        Client:     redisClient,
        QueueName:  "usage_processing_queue",
        BatchSize:  50,
        MaxRetries: 3,
    }),
})
```

### Budget Monitoring

Track usage via API:
```bash
# Get user usage stats
GET /v1/user/usage
GET /v1/user/usage/daily  
GET /v1/user/usage/monthly
```

## Rate Limiting

### Global Rate Limits

System-wide rate limiting (from `config.go`):

```yaml
rate_limit:
  enabled: true
  requests_per_minute: 600
  burst: 10
  global_rpm: 10000
  chat_completions_rpm: 5000
  completions_rpm: 3000
  embeddings_rpm: 2000
```

### Per-Model Rate Limits

Configure limits per model:

```yaml
model_list:
  - model_name: fast-gpt
    params:
      model: gpt-3.5-turbo
      api_key: ${OPENAI_API_KEY}
    rpm: 500      # requests per minute
    tpm: 100000   # tokens per minute
```

## Security Features

### Request Validation & Audit

All API requests are:
- Authenticated and authorized
- Rate limited per user/team
- Budget checked (async)
- Logged for audit

### Database Integration

Authentication integrates with PostgreSQL:
- User and team management
- API key storage (encrypted)
- Usage tracking and billing
- Audit logs

### Environment Variables

Required environment variables:

```bash
# Authentication
JWT_SECRET_KEY=your-super-secret-jwt-key-change-this
PLLM_MASTER_KEY=sk-master-dev-key-change-in-production

# Database (required for auth)  
DATABASE_URL=postgres://pllm:pllm@localhost:5432/pllm

# Redis (required for async budget system)
REDIS_URL=redis://localhost:6379

# Dex OIDC (optional)
DEX_ENABLED=true
DEX_ISSUER=http://localhost:5556/dex
DEX_CLIENT_ID=pllm-web
DEX_CLIENT_SECRET=pllm-web-secret
```

## API Endpoints

### Authentication Routes

```bash
# Public routes
POST /v1/register    # User registration
POST /v1/login       # User login  
POST /v1/refresh     # Refresh JWT token

# Protected user routes (require JWT)
GET  /v1/user/profile       # Get user profile
PUT  /v1/user/profile       # Update profile
PUT  /v1/user/password      # Change password
GET  /v1/user/keys          # List API keys
POST /v1/user/keys          # Create API key
DELETE /v1/user/keys/{id}   # Delete API key
```

### Protected API Routes

All `/v1/*` routes require authentication:
- `/v1/chat/completions`
- `/v1/completions` 
- `/v1/embeddings`
- `/v1/models`

## Admin Configuration

Default admin credentials (change in production):

```yaml
admin:
  username: admin
  password: changeme123!
  email: admin@pllm.io
```

Access admin UI at: `http://localhost:8080/ui`

## Production Setup

For production deployments:

1. **Enable HTTPS/TLS**
2. **Use strong JWT secrets**
3. **Configure Dex with your IdP**
4. **Set proper CORS origins**
5. **Enable audit logging**
6. **Use PostgreSQL with SSL**
7. **Secure Redis connection**

For detailed implementation, see [AUTH_IMPLEMENTATION_PLAN.md](AUTH_IMPLEMENTATION_PLAN.md).