# PLLM Deployment with Authentication

## Quick Start

1. **Copy environment configuration**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

2. **Start all services**
   ```bash
   docker-compose -f docker-compose.auth.yml up -d
   ```

3. **Access services**
   - Web UI: http://localhost:3000
   - API: http://localhost:8080
   - Admin API: http://localhost:8081
   - Dex (SSO): http://localhost:5556
   - Metrics: http://localhost:9090

## Initial Setup

### Database Seeding
The database will automatically seed with demo data on first startup:
- **Admin**: admin@pllm.local / admin123
- **Manager**: manager@pllm.local / user123
- **User**: user@pllm.local / user123
- **Demo**: demo@pllm.local / demo123

⚠️ **IMPORTANT**: Change default passwords in production!

### SSO Configuration

#### GitHub OAuth
1. Create OAuth App: https://github.com/settings/applications/new
2. Set Homepage URL: `http://localhost:3000`
3. Set Authorization callback URL: `http://localhost:5556/dex/callback`
4. Copy Client ID and Secret to `.env`

#### Google OAuth
1. Go to: https://console.cloud.google.com/apis/credentials
2. Create OAuth 2.0 Client ID
3. Add Authorized redirect URI: `http://localhost:5556/dex/callback`
4. Copy Client ID and Secret to `.env`

#### LDAP/SAML
Edit `dex/config.yaml` with your enterprise configuration.

## Authentication Methods

### 1. Master Key
For administrative operations:
```bash
curl -H "Authorization: Bearer sk-master-..." http://localhost:8080/api/...
```

### 2. Virtual Keys
API access with budget and rate limits:
```bash
curl -H "Authorization: Bearer sk-admin-full-access-..." http://localhost:8080/api/...
```

### 3. SSO (Web UI)
Users can login through:
- GitHub
- Google
- LDAP
- SAML
- Mock (for testing)

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Web UI    │────▶│    PLLM     │────▶│  PostgreSQL │
│  (React)    │     │   Service   │     │  (Users/    │
└─────────────┘     └─────────────┘     │   Teams)    │
                           │             └─────────────┘
                           │
                    ┌──────▼──────┐     ┌─────────────┐
                    │     Dex     │────▶│    Redis    │
                    │    (SSO)    │     │   (Cache)   │
                    └─────────────┘     └─────────────┘
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| Web UI | 3000 | React admin interface |
| PLLM API | 8080 | Main API endpoint |
| Admin API | 8081 | Administrative endpoints |
| Dex | 5556 | SSO provider |
| PostgreSQL | 5432 | Database |
| Redis | 6379 | Cache & rate limiting |
| Metrics | 9090 | Prometheus metrics |

## Management Commands

### View logs
```bash
docker-compose -f docker-compose.auth.yml logs -f [service]
```

### Restart service
```bash
docker-compose -f docker-compose.auth.yml restart [service]
```

### Stop all services
```bash
docker-compose -f docker-compose.auth.yml down
```

### Remove all data
```bash
docker-compose -f docker-compose.auth.yml down -v
```

## Production Considerations

1. **Security**
   - Change all default passwords
   - Use strong secrets for JWT and Master Key
   - Enable TLS/HTTPS
   - Configure firewall rules

2. **Database**
   - Use managed PostgreSQL service
   - Configure backups
   - Set appropriate connection pool sizes

3. **Redis**
   - Use Redis Cluster for HA
   - Configure persistence
   - Set memory limits

4. **Dex**
   - Use persistent storage (PostgreSQL)
   - Configure proper OAuth redirect URLs
   - Remove mock connector

5. **Monitoring**
   - Set up Prometheus/Grafana
   - Configure alerting
   - Enable distributed tracing