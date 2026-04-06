# Provider Profiles Design Spec

**Date:** 2026-04-06
**Status:** Approved
**Scope:** Backend CRUD for provider profiles + frontend Providers page + AddModel wizard integration

## Overview

Provider profiles allow users to save LLM provider credentials (API keys, endpoints, etc.) as reusable entities. Multiple models can reference the same provider profile instead of each storing credentials inline. This eliminates re-entering credentials when onboarding multiple models from the same provider.

## Constraints

- **Opt-in only** — Existing models with inline credentials continue working unchanged. No migration.
- **Optional reference** — New models can either reference a saved provider profile OR use inline credentials. Both paths supported.
- **Credentials only** — Provider profiles store type + credentials. No defaults for RPM/TPM, timeout, pricing. Models own their own config.

## Backend

### Provider Entity

Implement the existing unused `Provider` model in `internal/core/models/provider.go` as a first-class CRUD resource.

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | uint (PK, auto) | Primary key |
| `name` | string (required, unique) | User-friendly label, e.g. "Our OpenAI Account" |
| `type` | string (required) | Provider type: `openai`, `anthropic`, `azure`, `bedrock`, `vertex`, `openrouter` |
| `config` | JSONB (required) | Provider-specific credentials (see below) |
| `created_at` | timestamp | Auto-set |
| `updated_at` | timestamp | Auto-set |

**Config JSONB structure** (same fields as existing `ProviderConfig`, minus `type` and `model` which live elsewhere):

```json
{
  "api_key": "sk-...",
  "base_url": "https://...",
  "oauth_token": "...",
  "azure_endpoint": "https://...",
  "azure_deployment": "...",
  "api_version": "2024-06-01",
  "aws_region_name": "us-east-1",
  "aws_access_key_id": "...",
  "aws_secret_access_key": "...",
  "vertex_project": "...",
  "vertex_location": "us-central1"
}
```

Only the fields relevant to the provider `type` are populated. Others are omitted or empty.

### API Endpoints

All under `/api/admin/providers`:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/admin/providers` | List all provider profiles |
| `POST` | `/api/admin/providers` | Create a provider profile |
| `GET` | `/api/admin/providers/:id` | Get a single provider profile |
| `PUT` | `/api/admin/providers/:id` | Update a provider profile |
| `DELETE` | `/api/admin/providers/:id` | Delete a provider profile |

**Create/Update request body:**
```json
{
  "name": "Our OpenAI Account",
  "type": "openai",
  "config": {
    "api_key": "sk-..."
  }
}
```

**List response:**
```json
{
  "providers": [
    {
      "id": 1,
      "name": "Our OpenAI Account",
      "type": "openai",
      "config": {
        "api_key": "sk-****a8f2"
      },
      "model_count": 3,
      "created_at": "2026-04-06T...",
      "updated_at": "2026-04-06T..."
    }
  ]
}
```

**Important:** API key values in GET responses must be masked (show only last 4 characters). Full values are only accepted on POST/PUT, never returned.

**Delete protection:** `DELETE /api/admin/providers/:id` returns `409 Conflict` with `{"error": "Cannot delete provider: 3 models reference it", "model_count": 3}` if any models have `provider_id` pointing to it.

### UserModel Change

Add an optional `provider_id` column to `user_models` table:

```go
type UserModel struct {
    // ... existing fields ...
    ProviderID       *uint    `json:"provider_id,omitempty" gorm:"index"`
    Provider         *Provider `json:"provider,omitempty" gorm:"foreignKey:ProviderID"`
    ProviderConfigJSON string  `json:"provider_config,omitempty" gorm:"column:provider_config;type:jsonb"`
    // ... existing fields ...
}
```

- If `provider_id` is set: model uses credentials from the linked Provider record
- If `provider_id` is null: model uses its existing inline `provider_config` (backwards compatible)
- Both fields can coexist. `provider_id` takes precedence at runtime.

### CreateModelRequest Change

```go
type CreateModelRequest struct {
    // ... existing fields ...
    ProviderID *uint          `json:"provider_id,omitempty"`  // Reference saved provider
    Provider   *ProviderConfig `json:"provider,omitempty"`    // OR inline credentials
    // ... existing fields ...
}
```

Validation: exactly one of `provider_id` or `provider` must be set. If both or neither, return `400 Bad Request`.

When `provider_id` is set, the handler must:
1. Look up the Provider record
2. Verify it exists
3. Use its `type` and `config` for the model's provider configuration
4. Store the `provider_id` on the UserModel

### Runtime Credential Resolution

In the LLM service layer where provider configs are resolved for making API calls, add a resolution step:

```
if model.ProviderID != nil {
    // Load Provider record, merge type + config into ProviderConfig
    provider := loadProvider(model.ProviderID)
    config = buildProviderConfig(provider.Type, provider.Config, model.ProviderModel)
} else {
    // Use existing inline provider_config
    config = model.ProviderConfig
}
```

This is the only change to the runtime path. Everything downstream (the actual API calls to OpenAI/Anthropic/etc.) stays the same.

### Test Connection with Provider Profile

The existing `POST /api/admin/models/test` endpoint should also accept `provider_id`:

```json
{
  "provider_id": 1,
  "model": "gpt-4o"
}
```

OR the existing inline format. Same validation rule: one or the other.

## Frontend

### Providers Page (new)

**Sidebar:** Add "Providers" under the Management group (between "API Keys" and "Teams").

**Page layout:**
- Header: "Providers" title, subtitle "Manage saved LLM provider credentials", "Add Provider" button
- Table: Name, Type (with brand logo from `@/lib/provider-logos`), Models (count of models using this provider), Created, Actions (edit/delete)
- Empty state: "No providers saved. Add your first provider to reuse credentials across models."

**Create/Edit dialog:**
- Step 1: Name input + provider type selector (same card grid as AddModel step 1, with brand logos)
- Step 2: Provider-specific credential fields (same fields as AddModel step 2, contextual to provider type)
- Save button

**Delete confirmation:** Shows model count. If models reference this provider, show warning "X models use this provider. You must reassign or remove them first." and disable the delete button.

### AddModel Wizard Update (Step 2 — Authentication)

When the user reaches Step 2, if saved providers exist for the selected provider type:

**Top section: "Saved Credentials"**
- Show matching provider profiles as selectable cards
- Each card: provider profile name, masked API key preview (`sk-****a8f2`), model count badge
- Selecting a card sets `provider_id` on the form and skips the credential fields

**Divider: "Or enter new credentials"**

**Bottom section: current credential form (unchanged)**
- Same fields as today
- New checkbox at the bottom: "Save as provider profile" with a name input
- If checked, on model creation: first create the provider profile via API, then create the model with `provider_id`

### API Client Updates

Add to `web/src/lib/api.ts`:

```ts
// Provider CRUD
export const getProviders = () => api.get('/admin/providers').then(r => r.data);
export const getProvider = (id: number) => api.get(`/admin/providers/${id}`).then(r => r.data);
export const createProvider = (data: CreateProviderRequest) => api.post('/admin/providers', data).then(r => r.data);
export const updateProvider = (id: number, data: CreateProviderRequest) => api.put(`/admin/providers/${id}`, data).then(r => r.data);
export const deleteProvider = (id: number) => api.delete(`/admin/providers/${id}`).then(r => r.data);
```

### Types

Add to `web/src/types/api.ts`:

```ts
export interface ProviderProfile {
  id: number;
  name: string;
  type: string;
  config: Record<string, string>;
  model_count?: number;
  created_at: string;
  updated_at: string;
}

export interface CreateProviderRequest {
  name: string;
  type: string;
  config: Record<string, string>;
}
```

Update `CreateModelRequest`:
```ts
export interface CreateModelRequest {
  // ... existing fields ...
  provider_id?: number;  // Reference saved provider
  provider?: ProviderConfig;  // OR inline credentials
  // ... existing fields ...
}
```

## What Stays Unchanged

- Existing models with inline credentials — no migration, no changes
- All existing model CRUD operations — work as before
- The runtime LLM call path — only the credential resolution step is added
- EditModel page — can optionally add provider profile selection later, but not in scope for this spec
- Config file models (`config.yaml`) — unaffected, these use their own config format
