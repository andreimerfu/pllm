# Go Codebase Refactoring Plan

## Overview
This document outlines the comprehensive refactoring plan for reorganizing the PLLM Go codebase to improve structure, reduce directory depth, and create clear separation between reusable utilities and application-specific code.

## Current Structure Analysis
- **142 Go files** in internal/
- **64 files** in services/ (45% of codebase)
- **29 files** in handlers/
- Multiple nested directories with mixed responsibilities

## Target Architecture

```
pllm/
├── pkg/                    # Reusable, generic utilities
│   ├── retry/             # Retry logic with backoff
│   ├── circuitbreaker/    # Circuit breaker pattern
│   ├── cache/             # Generic caching utilities
│   └── loadbalancer/      # Load balancing algorithms
│
└── internal/              # Application-specific code
    ├── core/              # Foundation components
    │   ├── auth/          # Authentication & authorization
    │   ├── config/        # Configuration management
    │   ├── database/      # DB connection & migrations
    │   └── models/        # Data models & schemas
    │
    ├── api/               # HTTP layer
    │   ├── handlers/      # HTTP request handlers
    │   ├── router/        # Route configuration
    │   ├── docs/          # Swagger documentation
    │   └── ui/            # Embedded UI assets
    │
    ├── infrastructure/    # Cross-cutting concerns
    │   ├── middleware/    # HTTP middleware chain
    │   ├── logger/        # Logging infrastructure
    │   └── testutil/      # Test utilities
    │
    └── services/          # Business logic (grouped by domain)
        ├── llm/           # LLM provider management
        │   ├── providers/ # OpenAI, Anthropic, etc.
        │   ├── models/    # Model registry & config
        │   └── realtime/  # WebSocket/streaming
        │
        ├── monitoring/    # Observability
        │   ├── metrics/   # Metrics collection
        │   ├── audit/     # Audit logging
        │   └── ratelimit/ # Rate limiting
        │
        ├── data/          # Data layer services
        │   ├── redis/     # Redis operations
        │   ├── cache/     # Application caching
        │   └── budget/    # Budget tracking
        │
        └── integrations/  # External integrations
            ├── guardrails/# Content filtering
            ├── team/      # Team management
            └── key/       # API key management
```

## Implementation Phases

### Phase 1: Analysis & Assessment ✅
**Status**: COMPLETED
- Analyzed package dependencies
- Identified reusable components
- Categorized by responsibility
- Documented current structure

### Phase 2: Directory Structure Creation
**Status**: IN PROGRESS
- Create pkg/ directory at project root
- Create internal/core/ for foundation components
- Create internal/api/ for HTTP layer
- Create internal/infrastructure/ for middleware
- Reorganize internal/services/ by business domain

### Phase 3: Move Generic Utilities to pkg/
**Files to move:**
- internal/services/retry/ → pkg/retry/
- internal/services/circuitbreaker/ → pkg/circuitbreaker/
- internal/services/loadbalancer/ → pkg/loadbalancer/
- Generic cache utilities → pkg/cache/

### Phase 4: Reorganize Core Components
**Moves:**
- internal/auth/ → internal/core/auth/
- internal/config/ → internal/core/config/
- internal/database/ → internal/core/database/
- internal/models/ → internal/core/models/

### Phase 5: Consolidate API Layer
**Moves:**
- internal/handlers/ → internal/api/handlers/
- internal/router/ → internal/api/router/
- internal/docs/ → internal/api/docs/
- internal/ui/ → internal/api/ui/

### Phase 6: Group Business Services
**Service groupings:**
```
internal/services/llm/
  - providers/ (OpenAI, Anthropic, Azure, etc.)
  - models/ (model registry and config)
  - realtime/ (WebSocket handling)

internal/services/monitoring/
  - metrics/ (collection and staging)
  - audit/ (audit logging)
  - ratelimit/ (rate limiting)

internal/services/data/
  - redis/ (Redis operations)
  - cache/ (application caching)
  - budget/ (budget management)

internal/services/integrations/
  - guardrails/ (content filtering)
  - team/ (team management)
  - key/ (API key management)
```

### Phase 7: Update Import Statements
1. Update imports for pkg/ packages
2. Update imports for internal/core/
3. Update imports for internal/api/
4. Update imports for reorganized services
5. Use automated find/replace with verification

### Phase 8: Validation & Testing ✅
**Status**: COMPLETED
**Validation steps:**
1. `go mod tidy` - clean dependencies
2. `go build ./...` - verify compilation
3. `go test ./...` - run all tests
4. `make lint` - check code quality
5. `make docker-up` - verify Docker builds
6. Manual smoke testing of key endpoints

### Phase 9: Documentation Updates
**Updates needed:**
1. Update CLAUDE.md with new structure
2. Update README.md project structure section
3. Create ARCHITECTURE.md documenting new organization
4. Update import examples in documentation
5. Document migration rationale

## Migration Log

### Phase 2: Directory Creation
- [ ] Create pkg/ directory
- [ ] Create internal/core/ subdirectories
- [ ] Create internal/api/ subdirectories
- [ ] Create internal/infrastructure/ subdirectories
- [ ] Create internal/services/ domain subdirectories

### Phase 3: pkg/ Migration
- [ ] Move retry package
- [ ] Move circuitbreaker package
- [ ] Move loadbalancer package
- [ ] Create generic cache utilities

## Benefits
- **Clear separation** between reusable (pkg/) and app-specific (internal/)
- **Reduced directory depth** - easier navigation
- **Domain-based grouping** - related functionality together
- **Cleaner imports** - more intuitive paths
- **Better testability** - clear boundaries between layers
- **Easier onboarding** - logical structure for new developers

## Rollback Strategy
Each phase is committed separately, allowing for easy rollback if issues arise. Git branches will be used for each major phase.