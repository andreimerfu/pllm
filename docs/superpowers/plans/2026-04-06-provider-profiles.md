# Provider Profiles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to save LLM provider credentials as reusable profiles that multiple models can reference, eliminating re-entry of API keys when onboarding models from the same provider.

**Architecture:** New `ProviderProfile` GORM model with CRUD API. `UserModel` gets an optional `ProviderProfileID` foreign key. At runtime, if a model references a profile, credentials are loaded from it. Frontend gets a new Providers management page and AddModel wizard integration.

**Tech Stack:** Go (Chi router, GORM, Zap logger), React/TypeScript (TanStack Query, Radix UI, Iconify)

**Spec:** `docs/superpowers/specs/2026-04-06-provider-profiles-design.md`

---

## File Structure

### Backend — New Files
- `internal/core/models/provider_profile.go` — ProviderProfile GORM model
- `internal/services/integrations/provider/service.go` — Provider profile CRUD service

### Backend — Modified Files
- `internal/core/database/migrate.go` — Add ProviderProfile to auto-migration
- `internal/api/handlers/admin/providers.go` — Replace stubs with real CRUD handlers
- `internal/api/router/admin.go` — Wire provider profile routes with DB dependency
- `internal/api/handlers/admin/models.go` — Add `provider_profile_id` to CreateModelRequest, resolve credentials
- `internal/core/models/user_model.go` — Add optional `ProviderProfileID` field
- `internal/services/integrations/model/service.go` — Update ConvertToModelInstance to resolve profile credentials

### Frontend — New Files
- `web/src/pages/Providers.tsx` — Provider profiles management page
- `web/src/hooks/useProviders.ts` — TanStack Query hooks for provider CRUD

### Frontend — Modified Files
- `web/src/lib/api.ts` — Add provider API functions
- `web/src/types/api.ts` — Add ProviderProfile types, update CreateModelRequest
- `web/src/pages/AddModel.tsx` — Add saved provider selection in Step 2
- `web/src/components/app-sidebar.tsx` — Add Providers nav item
- `web/src/App.tsx` — Add /providers route

---

## Tasks

### Task 1: Create ProviderProfile Model

**Files:**
- Create: `internal/core/models/provider_profile.go`
- Modify: `internal/core/database/migrate.go`

- [ ] **Step 1: Create the ProviderProfile model**

Create `internal/core/models/provider_profile.go`:

```go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// ProviderProfile stores reusable provider credentials.
// Multiple UserModels can reference a single profile.
type ProviderProfile struct {
	BaseModel
	Name   string                    `gorm:"uniqueIndex;not null" json:"name"`
	Type   string                    `gorm:"not null;index" json:"type"`
	Config ProviderProfileConfigJSON `gorm:"type:jsonb;not null" json:"config"`
}

// ProviderProfileConfigJSON stores provider-specific credentials as JSONB.
type ProviderProfileConfigJSON struct {
	APIKey             string `json:"api_key,omitempty"`
	BaseURL            string `json:"base_url,omitempty"`
	OAuthToken         string `json:"oauth_token,omitempty"`
	AzureEndpoint      string `json:"azure_endpoint,omitempty"`
	AzureDeployment    string `json:"azure_deployment,omitempty"`
	APIVersion         string `json:"api_version,omitempty"`
	AWSRegionName      string `json:"aws_region_name,omitempty"`
	AWSAccessKeyID     string `json:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string `json:"aws_secret_access_key,omitempty"`
	VertexProject      string `json:"vertex_project,omitempty"`
	VertexLocation     string `json:"vertex_location,omitempty"`
}

func (c ProviderProfileConfigJSON) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *ProviderProfileConfigJSON) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan ProviderProfileConfigJSON: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, c)
}
```

- [ ] **Step 2: Add ProviderProfile to auto-migration**

In `internal/core/database/migrate.go`, add `&models.ProviderProfile{}` to the `AutoMigrate` call:

```go
err := db.AutoMigrate(
    &models.User{},
    &models.Team{},
    &models.TeamMember{},
    &models.Key{},
    &models.Budget{},
    &models.Usage{},
    &models.Audit{},
    &models.UserModel{},
    &models.Route{},
    &models.RouteModel{},
    &models.ProviderProfile{},
)
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/core/models/provider_profile.go internal/core/database/migrate.go
git commit -m "feat: add ProviderProfile model and database migration"
```

---

### Task 2: Create Provider Profile Service

**Files:**
- Create: `internal/services/integrations/provider/service.go`

- [ ] **Step 1: Create the provider service**

Create `internal/services/integrations/provider/service.go`:

```go
package provider

import (
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/teilomillet/pllm/internal/core/models"
)

type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

func (s *Service) List() ([]models.ProviderProfile, error) {
	var profiles []models.ProviderProfile
	if err := s.db.Order("created_at DESC").Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("failed to list provider profiles: %w", err)
	}
	return profiles, nil
}

func (s *Service) ListByType(providerType string) ([]models.ProviderProfile, error) {
	var profiles []models.ProviderProfile
	if err := s.db.Where("type = ?", providerType).Order("name ASC").Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("failed to list provider profiles by type: %w", err)
	}
	return profiles, nil
}

func (s *Service) Get(id uuid.UUID) (*models.ProviderProfile, error) {
	var profile models.ProviderProfile
	if err := s.db.First(&profile, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("provider profile not found: %w", err)
	}
	return &profile, nil
}

func (s *Service) Create(profile *models.ProviderProfile) error {
	if err := s.db.Create(profile).Error; err != nil {
		return fmt.Errorf("failed to create provider profile: %w", err)
	}
	return nil
}

func (s *Service) Update(id uuid.UUID, updates map[string]interface{}) (*models.ProviderProfile, error) {
	var profile models.ProviderProfile
	if err := s.db.First(&profile, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("provider profile not found: %w", err)
	}
	if err := s.db.Model(&profile).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update provider profile: %w", err)
	}
	// Reload to get updated values
	if err := s.db.First(&profile, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("failed to reload provider profile: %w", err)
	}
	return &profile, nil
}

func (s *Service) Delete(id uuid.UUID) error {
	// Check if any models reference this profile
	var count int64
	if err := s.db.Model(&models.UserModel{}).Where("provider_profile_id = ?", id).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check model references: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete provider profile: %d models reference it", count)
	}

	result := s.db.Delete(&models.ProviderProfile{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete provider profile: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("provider profile not found")
	}
	return nil
}

func (s *Service) GetModelCount(id uuid.UUID) (int64, error) {
	var count int64
	if err := s.db.Model(&models.UserModel{}).Where("provider_profile_id = ?", id).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/integrations/provider/service.go
git commit -m "feat: add provider profile service with CRUD operations"
```

---

### Task 3: Implement Provider Profile API Handlers

**Files:**
- Modify: `internal/api/handlers/admin/providers.go`

- [ ] **Step 1: Replace the stub handler with real implementation**

Read `internal/api/handlers/admin/providers.go` first. Then replace the entire file with a real CRUD handler. The handler must:

1. Have `db *gorm.DB` and `service *providerService.Service` fields
2. Update `NewProviderHandler` to accept `*gorm.DB` as parameter and create the service
3. Implement `ListProviders` — returns all profiles with model counts, masks API keys in responses
4. Implement `CreateProvider` — validates name + type + config, creates via service
5. Implement `GetProvider` — returns single profile with masked keys
6. Implement `UpdateProvider` — updates name/type/config
7. Implement `DeleteProvider` — checks model count, returns 409 if referenced

**Request/response types to define in the file:**

```go
type CreateProviderProfileRequest struct {
	Name   string                             `json:"name"`
	Type   string                             `json:"type"`
	Config models.ProviderProfileConfigJSON   `json:"config"`
}

type providerProfileResponse struct {
	ID         string                             `json:"id"`
	Name       string                             `json:"name"`
	Type       string                             `json:"type"`
	Config     models.ProviderProfileConfigJSON   `json:"config"`
	ModelCount int64                              `json:"model_count"`
	CreatedAt  string                             `json:"created_at"`
	UpdatedAt  string                             `json:"updated_at"`
}
```

**Key patterns to follow from the existing models handler:**
- Use `h.sendResponse(w, data, http.StatusOK)` and `h.sendError(w, message, http.StatusXxx)`
- Mask secrets: replace API key with `maskSecret(key)` — show only last 4 chars
- Parse path params with `chi.URLParam(r, "providerID")`
- Parse UUID with `uuid.Parse(idStr)`
- Decode request body with `json.NewDecoder(r.Body).Decode(&req)`

**Secret masking function:**

```go
func maskSecret(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}
```

Apply masking to `api_key`, `oauth_token`, `aws_access_key_id`, `aws_secret_access_key` in response conversion.

**Valid provider types:** `openai`, `anthropic`, `azure`, `bedrock`, `vertex`, `openrouter`

- [ ] **Step 2: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handlers/admin/providers.go
git commit -m "feat: implement provider profile CRUD API handlers"
```

---

### Task 4: Wire Provider Routes and Update Model Handler

**Files:**
- Modify: `internal/api/router/admin.go`
- Modify: `internal/api/handlers/admin/models.go`
- Modify: `internal/core/models/user_model.go`
- Modify: `internal/services/integrations/model/service.go`

- [ ] **Step 1: Add ProviderProfileID to UserModel**

In `internal/core/models/user_model.go`, add to the `UserModel` struct:

```go
ProviderProfileID *uuid.UUID       `gorm:"type:uuid;index" json:"provider_profile_id,omitempty"`
ProviderProfile   *ProviderProfile `gorm:"foreignKey:ProviderProfileID" json:"provider_profile,omitempty"`
```

Add these after the existing `ProviderConfig` field.

- [ ] **Step 2: Update CreateModelRequest in models.go**

In `internal/api/handlers/admin/models.go`, add to `CreateModelRequest`:

```go
ProviderProfileID *string `json:"provider_profile_id,omitempty"`
```

Then in the `CreateModel` handler, after decoding the request, add logic to resolve provider profile:

```go
// If provider_profile_id is set, load credentials from the profile
if req.ProviderProfileID != nil && *req.ProviderProfileID != "" {
    profileID, err := uuid.Parse(*req.ProviderProfileID)
    if err != nil {
        h.sendError(w, "Invalid provider_profile_id", http.StatusBadRequest)
        return
    }
    var profile models.ProviderProfile
    if err := h.db.First(&profile, "id = ?", profileID).Error; err != nil {
        h.sendError(w, "Provider profile not found", http.StatusNotFound)
        return
    }
    // Build provider config from profile
    req.Provider = models.ProviderConfigJSON{
        Type:               profile.Type,
        Model:              req.Provider.Model, // Model comes from the request
        APIKey:             profile.Config.APIKey,
        BaseURL:            profile.Config.BaseURL,
        OAuthToken:         profile.Config.OAuthToken,
        AzureEndpoint:      profile.Config.AzureEndpoint,
        AzureDeployment:    profile.Config.AzureDeployment,
        APIVersion:         profile.Config.APIVersion,
        AWSRegionName:      profile.Config.AWSRegionName,
        AWSAccessKeyID:     profile.Config.AWSAccessKeyID,
        AWSSecretAccessKey: profile.Config.AWSSecretAccessKey,
        VertexProject:      profile.Config.VertexProject,
        VertexLocation:     profile.Config.VertexLocation,
    }
    userModel.ProviderProfileID = &profileID
}
```

Insert this before the existing provider config validation.

- [ ] **Step 3: Update the router to pass DB to ProviderHandler**

In `internal/api/router/admin.go`:
1. Read the file to find how `NewProviderHandler` is called
2. Update the call to pass `db` as the second argument: `NewProviderHandler(logger, db)`
3. Ensure the `/providers` route group is properly registered

- [ ] **Step 4: Update model service ConvertToModelInstance**

In `internal/services/integrations/model/service.go`, in the `ConvertToModelInstance` method, add profile resolution:

```go
// If model references a provider profile, load and use its credentials
if um.ProviderProfileID != nil {
    var profile models.ProviderProfile
    if err := s.db.First(&profile, "id = ?", um.ProviderProfileID).Error; err == nil {
        // Override credentials from profile
        providerConfig := um.ProviderConfig
        providerConfig.Type = profile.Type
        providerConfig.APIKey = profile.Config.APIKey
        providerConfig.BaseURL = profile.Config.BaseURL
        providerConfig.OAuthToken = profile.Config.OAuthToken
        providerConfig.AzureEndpoint = profile.Config.AzureEndpoint
        providerConfig.AzureDeployment = profile.Config.AzureDeployment
        providerConfig.APIVersion = profile.Config.APIVersion
        providerConfig.AWSRegionName = profile.Config.AWSRegionName
        providerConfig.AWSAccessKeyID = profile.Config.AWSAccessKeyID
        providerConfig.AWSSecretAccessKey = profile.Config.AWSSecretAccessKey
        providerConfig.VertexProject = profile.Config.VertexProject
        providerConfig.VertexLocation = profile.Config.VertexLocation
        um.ProviderConfig = providerConfig
    }
}
```

Add this at the beginning of `ConvertToModelInstance`, before the existing env var expansion.

- [ ] **Step 5: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 6: Run existing tests**

```bash
go test ./internal/... -v -short
```

Fix any failures caused by the new fields.

- [ ] **Step 7: Commit**

```bash
git add internal/api/router/admin.go internal/api/handlers/admin/models.go internal/core/models/user_model.go internal/services/integrations/model/service.go
git commit -m "feat: wire provider profiles into model creation and runtime resolution"
```

---

### Task 5: Frontend — API Client and Types

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/types/api.ts`
- Create: `web/src/hooks/useProviders.ts`

- [ ] **Step 1: Add types**

In `web/src/types/api.ts`, add:

```ts
export interface ProviderProfile {
  id: string;
  name: string;
  type: string;
  config: {
    api_key?: string;
    base_url?: string;
    oauth_token?: string;
    azure_endpoint?: string;
    azure_deployment?: string;
    api_version?: string;
    aws_region_name?: string;
    aws_access_key_id?: string;
    aws_secret_access_key?: string;
    vertex_project?: string;
    vertex_location?: string;
  };
  model_count?: number;
  created_at: string;
  updated_at: string;
}

export interface CreateProviderProfileRequest {
  name: string;
  type: string;
  config: Record<string, string>;
}
```

Also add `provider_profile_id?: string` to the existing `CreateModelRequest` interface.

- [ ] **Step 2: Add API functions**

In `web/src/lib/api.ts`, add:

```ts
// Provider Profiles
export const getProviderProfiles = () => api.get('/admin/providers').then(r => r.data.providers || r.data);
export const getProviderProfile = (id: string) => api.get(`/admin/providers/${id}`).then(r => r.data);
export const createProviderProfile = (data: any) => api.post('/admin/providers', data).then(r => r.data);
export const updateProviderProfile = (id: string, data: any) => api.put(`/admin/providers/${id}`, data).then(r => r.data);
export const deleteProviderProfile = (id: string) => api.delete(`/admin/providers/${id}`).then(r => r.data);
```

- [ ] **Step 3: Create useProviders hook**

Create `web/src/hooks/useProviders.ts`:

```ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getProviderProfiles,
  createProviderProfile,
  updateProviderProfile,
  deleteProviderProfile,
} from '@/lib/api';

export function useProviderProfiles() {
  return useQuery({
    queryKey: ['provider-profiles'],
    queryFn: getProviderProfiles,
  });
}

export function useCreateProviderProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createProviderProfile,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['provider-profiles'] });
    },
  });
}

export function useUpdateProviderProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => updateProviderProfile(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['provider-profiles'] });
    },
  });
}

export function useDeleteProviderProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteProviderProfile,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['provider-profiles'] });
    },
  });
}
```

- [ ] **Step 4: Verify build**

```bash
cd web && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add web/src/types/api.ts web/src/lib/api.ts web/src/hooks/useProviders.ts
git commit -m "feat(ui): add provider profile types, API client, and query hooks"
```

---

### Task 6: Frontend — Providers Management Page

**Files:**
- Create: `web/src/pages/Providers.tsx`
- Modify: `web/src/components/app-sidebar.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create the Providers page**

Create `web/src/pages/Providers.tsx` with:

1. **Page header:** "Providers" title, "Manage saved LLM provider credentials" subtitle, "Add Provider" button
2. **Table:** Name, Type (with brand logo via `getProviderLogo`), Models (count), Created (relative time), Actions (edit/delete)
3. **Empty state:** "No providers saved" with CTA
4. **Create/Edit dialog:** Name input, provider type selector (card grid with logos like AddModel), provider-specific credential fields (same fields as AddModel Step 2, contextual to type)
5. **Delete confirmation:** Shows model count, disabled if models reference the profile

Use `useProviderProfiles`, `useCreateProviderProfile`, `useUpdateProviderProfile`, `useDeleteProviderProfile` from `@/hooks/useProviders`.

Use `Icon` from `@iconify/react`, `icons` from `@/lib/icons`, `getProviderLogo` from `@/lib/provider-logos`.

Use existing UI components: Button, Input, Label, Dialog, Table, Badge, Select, Card.

Follow the same PROVIDERS array from AddModel.tsx for type options and credential field rendering.

- [ ] **Step 2: Add Providers to sidebar navigation**

In `web/src/components/app-sidebar.tsx`, add a "Providers" nav item to the Management group, between "API Keys" and "Teams":

```ts
{ title: "Providers", url: "/providers", icon: icons.globe },
```

- [ ] **Step 3: Add route to App.tsx**

In `web/src/App.tsx`, add:

```tsx
import Providers from "./pages/Providers";
// ...
<Route path="/providers" element={<Providers />} />
```

Add it in the protected routes section alongside other management pages.

- [ ] **Step 4: Verify build**

```bash
cd web && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/Providers.tsx web/src/components/app-sidebar.tsx web/src/App.tsx
git commit -m "feat(ui): add Providers management page with CRUD"
```

---

### Task 7: Frontend — AddModel Wizard Integration

**Files:**
- Modify: `web/src/pages/AddModel.tsx`

- [ ] **Step 1: Update AddModel Step 2 to show saved providers**

Read `web/src/pages/AddModel.tsx`. In the `renderAuthStep` function, add at the top:

1. Import and use `useProviderProfiles` to fetch saved profiles
2. Filter profiles by the selected provider type
3. If matching profiles exist, render a "Saved Credentials" section above the existing form:
   - Each saved profile shown as a selectable card: profile name, masked API key, model count badge
   - Selecting a card sets a `selectedProfileId` state variable and hides the credential form
   - A "Use different credentials" link to deselect and show the form

4. Below the existing credential form, add a checkbox: "Save these credentials for reuse"
   - When checked, show a "Profile name" input
   - On model creation (handleSubmit), if checked: first call `createProviderProfile` with the credentials, then create the model with `provider_profile_id`

**New state variables needed:**

```ts
const [selectedProfileId, setSelectedProfileId] = useState<string | null>(null);
const [saveAsProfile, setSaveAsProfile] = useState(false);
const [profileName, setProfileName] = useState("");
```

**Update handleSubmit:** If `selectedProfileId` is set, send `provider_profile_id` in the request instead of inline credentials. If `saveAsProfile` is checked, create the profile first, then use its ID.

**Update stepValid for step 2:** Valid if `selectedProfileId` is set OR if the existing `providerValid` check passes.

**Update the ProgressRail preview for step 2:** Show the profile name if a saved profile is selected.

- [ ] **Step 2: Verify build**

```bash
cd web && npm run build
```

- [ ] **Step 3: Test the full flow manually**

1. Start the app: `make dev`
2. Go to Providers page → Create a provider profile for OpenRouter
3. Go to Add Model → Select OpenRouter → Step 2 should show saved credentials
4. Select the saved profile → Continue through the wizard → Create model
5. Verify the model works

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/AddModel.tsx
git commit -m "feat(ui): integrate provider profiles into AddModel wizard"
```

---

## Post-Implementation Verification

1. Create a provider profile via the Providers page
2. Add a model using the saved profile — credentials should auto-fill
3. Add a second model from the same provider — should see saved profile in Step 2
4. Try to delete a provider profile that has models — should be blocked
5. Verify existing models with inline credentials still work
6. Test connection using a saved profile
