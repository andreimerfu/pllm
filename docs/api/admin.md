# Admin Endpoints

Everything under `/api/admin/*` requires admin-level auth (master key, or a
JWT for a user with admin role). This page is a cross-cutting catalog;
detailed shapes live with each feature's documentation.

## Auth flow

```
POST /api/admin/auth/login                  Password login → admin JWT
POST /api/admin/auth/master-key             Master key check
GET  /api/admin/auth/validate               Is this token still good?
POST /api/admin/auth/token                  Dex code → pllm JWT
GET  /api/admin/auth/userinfo               Dex userinfo passthrough
GET  /api/admin/auth/permissions            What can this caller do?
```

## Users & teams

```
GET    /api/admin/users
POST   /api/admin/users
GET    /api/admin/users/{id}
PUT    /api/admin/users/{id}
DELETE /api/admin/users/{id}

GET    /api/admin/teams
POST   /api/admin/teams
GET    /api/admin/teams/{id}
PUT    /api/admin/teams/{id}
DELETE /api/admin/teams/{id}

POST   /api/admin/teams/{id}/members        Add a user to a team
DELETE /api/admin/teams/{id}/members/{user_id}
```

## Keys

```
GET    /api/admin/keys
POST   /api/admin/keys                      Mint a new key
GET    /api/admin/keys/{id}
PUT    /api/admin/keys/{id}                 Toggle flags, update limits
DELETE /api/admin/keys/{id}                 Revoke
```

See [API Keys & Permissions](/security/keys) for payload details.

## LLM config

```
GET    /api/admin/models                    Live model list + health
POST   /api/admin/models                    Add a model (hot-reload)
PUT    /api/admin/models/{id}
DELETE /api/admin/models/{id}

GET    /api/admin/providers                 Configured provider profiles
POST   /api/admin/providers
PUT    /api/admin/providers/{id}
DELETE /api/admin/providers/{id}

GET    /api/admin/routes                    Routing config
POST   /api/admin/routes
PUT    /api/admin/routes/{id}
DELETE /api/admin/routes/{id}

GET    /api/admin/models/stats              Per-model: latency, success rate
```

## MCP gateway

```
GET    /api/admin/mcp/servers
POST   /api/admin/mcp/servers               Register an MCP backend
GET    /api/admin/mcp/servers/{id}
PUT    /api/admin/mcp/servers/{id}
DELETE /api/admin/mcp/servers/{id}

POST   /api/admin/mcp/tools/invoke          Direct tool call for testing
```

## Registry writes

```
POST   /api/admin/registry/servers          Publish or update a server
DELETE /api/admin/registry/servers/{name}/versions/{version}

POST   /api/admin/registry/agents           Same shape for agents
DELETE /api/admin/registry/agents/{name}/versions/{version}

POST   /api/admin/registry/skills
DELETE /api/admin/registry/skills/{name}/versions/{version}

POST   /api/admin/registry/prompts
DELETE /api/admin/registry/prompts/{name}/versions/{version}

POST   /api/admin/registry/import           npm / PyPI bulk import
POST   /api/admin/registry/enrichment/rescan  Re-queue all servers for OSV scan
```

## Deployments

```
GET    /api/admin/deployments               List MCP server deployments
POST   /api/admin/deployments               Deploy a registry server
GET    /api/admin/deployments/{id}
DELETE /api/admin/deployments/{id}          Undeploy
POST   /api/admin/deployments/{id}/status   Force K8s status refresh
GET    /api/admin/deployments/events        Operational event feed

POST   /api/admin/deployments/{id}/oom/bump
POST   /api/admin/deployments/{id}/oom/reset
PUT    /api/admin/deployments/{id}/oom/autoscale    Toggle per-deployment

GET    /api/admin/agents                    List agent deployments
POST   /api/admin/agents                    Deploy an agent
GET    /api/admin/agents/{id}
DELETE /api/admin/agents/{id}               Undeploy
POST   /api/admin/agents/{id}/invoke        Direct invoke for testing
```

## Use Cases (MRM)

```
GET    /api/admin/use-cases                 List
POST   /api/admin/use-cases                 Create
GET    /api/admin/use-cases/{id}
PUT    /api/admin/use-cases/{id}
DELETE /api/admin/use-cases/{id}

GET    /api/admin/use-cases/events          Scope event audit log
POST   /api/admin/use-cases/{id}/recompute  Force Reconciler run
```

Full details: [Governance](/governance/overview).

## Webhooks

```
GET    /api/admin/webhooks
POST   /api/admin/webhooks                  Subscribe
GET    /api/admin/webhooks/{id}
PUT    /api/admin/webhooks/{id}
DELETE /api/admin/webhooks/{id}
POST   /api/admin/webhooks/{id}/test        Fire a test delivery
```

Full details: [Webhooks](/governance/webhooks).

## Guardrails

```
GET  /api/admin/guardrails                          List + status
GET  /api/admin/guardrails/{guardrailName}
GET  /api/admin/guardrails/{guardrailName}/stats
GET  /api/admin/guardrails/{guardrailName}/health
POST /api/admin/guardrails/test                     Dry-run one message
```

Full details: [Guardrails](/security/guardrails).

## Usage & audit

```
GET  /api/admin/usage                       All users
GET  /api/admin/usage?user_id=...
GET  /api/admin/usage?team_id=...
GET  /api/admin/usage?key_id=...

GET  /api/admin/audit                       Audit events
GET  /api/admin/audit/export                CSV dump
```

## System

```
GET  /api/admin/system                      Version, uptime, feature flags
GET  /api/admin/system/feature-flags        What's enabled (MRM, MCP, etc.)
GET  /api/admin/config                      Sanitized current config
```

## Errors

Admin endpoints use the same error shape as public endpoints:

```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error | permission_error | not_found | ...",
    "code": "..."
  }
}
```

`permission_error` is returned for non-admin callers on admin routes, not
`authentication_error`.
