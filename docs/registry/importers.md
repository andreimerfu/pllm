# npm / PyPI Importers

pllm can bulk-ingest MCP server packages from **npm** and **PyPI** into the
registry. This is how you populate a fresh cluster with hundreds of public
MCP servers without publishing each one by hand.

## How it works

```mermaid
graph LR
    Trigger[POST /api/admin/registry/import<br/>npm | pypi] --> Fetch
    Fetch[Fetch package metadata] --> Filter[Keep MCP-tagged packages]
    Filter --> Map[Map to RegistryServer shape]
    Map --> Dedupe[Insert or skip if name+version exists]
    Dedupe --> Enrich[Queue OSV enrichment]
```

- **npm**: queries the registry for packages with the keyword `model-context-protocol`
- **PyPI**: queries for the `model-context-protocol` classifier

Packages that look MCP-shaped but don't publish the keyword are skipped —
imports err toward precision over recall. You can always publish manually.

## Namespace

Imported artifacts land in a namespaced section so they don't clutter curated
ones:

| Source | Namespace |
|---|---|
| npm | `npm.import/<package-name>` |
| PyPI | `pypi.import/<package-name>` |

This keeps the curated namespace (`pllm.io/*`, `official/*`, or whatever your
org uses) clean. The UI segregates them into a separate tab by default.

## Running an import

```bash
curl -X POST http://localhost:8080/api/admin/registry/import \
  -H "Authorization: Bearer $MASTER" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "npm",
    "limit": 100,
    "since": "2026-01-01T00:00:00Z"
  }'
```

| Field | Type | Purpose |
|---|---|---|
| `source` | `"npm"` \| `"pypi"` | Which registry to pull from |
| `limit` | int | Max packages to fetch this run |
| `since` | ISO date | Only packages updated on or after this date |
| `dry_run` | bool | Fetch + map but don't write |

Returns a summary with counts:

```json
{
  "source": "npm",
  "fetched": 247,
  "inserted": 89,
  "skipped_existing": 155,
  "skipped_invalid": 3,
  "duration_ms": 4231
}
```

## Visibility of imported artifacts

Imports default to `visibility: "public"`. If you want them private by default
(e.g., for review before exposing), run with `--visibility=private` and promote
individually.

## Updating imports

Re-running the importer is idempotent:

- New versions of an existing package are inserted (is_latest bookkeeping)
- Existing `name+version` combos are skipped
- Deleted upstream packages are **not** deleted from pllm — this is an append-only import

To remove a stale import, use the regular delete endpoint:

```bash
curl -X DELETE \
  "http://localhost:8080/api/admin/registry/servers/npm.import/some-deprecated-pkg/versions/1.0.0" \
  -H "Authorization: Bearer $MASTER"
```

## Scheduled imports

pllm doesn't ship a scheduler — use your own (cron, systemd timer, Kubernetes
CronJob) to run the import nightly or weekly. A one-liner:

```bash
0 2 * * *   curl -s -X POST http://pllm/api/admin/registry/import \
              -H "Authorization: Bearer $MASTER" \
              -d '{"source":"npm","since":"$(date -u -d '1 day ago' +%FT%TZ)"}'
```

## When not to import

- You only need a handful of specific servers — publish them manually
- You have strict supply-chain rules — review each addition before it hits the registry
- Your network can't reach npm/PyPI — import on a box that can, export JSON, `POST` back

The importer is a convenience, not a requirement.
