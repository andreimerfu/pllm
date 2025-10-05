# pLLM CLI

A comprehensive command-line interface for managing pLLM users, teams, API keys, and budgets.

## Features

- **Dual Access Modes**: Support for both direct database access (when running on the server) and remote API access
- **User Management**: Create, list, update, and delete users
- **Team Management**: Manage teams, add/remove members, set budgets
- **API Key Management**: Generate, list, revoke, and monitor API keys
- **Budget Management**: Set, monitor, and reset budgets for users, teams, and keys
- **Flexible Output**: Support for both table and JSON output formats
- **Configuration**: File-based or environment variable configuration

## Installation

### Build from Source

```bash
go build -o bin/pllm ./cmd/pllm
```

### Install to System Path

```bash
go install ./cmd/pllm
```

## Configuration

The CLI can be configured via:

1. Configuration file (`.pllm.yaml` in home directory or specified with `--config`)
2. Environment variables
3. Command-line flags

### Configuration File Example

Copy `.pllm.example.yaml` to `~/.pllm.yaml`:

```yaml
database:
  url: "postgres://pllm:pllm@localhost:5432/pllm?sslmode=disable"

api:
  url: "https://your-pllm-instance.com"
  key: "your-api-key-here"

output:
  json: false
  verbose: false
```

### Environment Variables

- `PLLM_DB_URL`: Database connection URL
- `PLLM_API_URL`: API base URL
- `PLLM_API_KEY`: API authentication key

## Usage Examples

### User Management

```bash
# Create a new user
pllm user create --email user@example.com --role admin --max-budget 1000

# List all users
pllm user list

# List users in a specific team
pllm user list --team-id <team-id>

# Get user details
pllm user get <user-id>

# Update user role
pllm user update <user-id> --role manager

# Delete user
pllm user delete <user-id>
```

### Team Management

```bash
# Create a new team
pllm team create --name "Engineering" --description "Engineering team" --max-budget 5000

# List teams
pllm team list

# Get team details
pllm team get <team-id>

# Add user to team
pllm team add-user <team-id> --user-id <user-id> --role member

# Remove user from team
pllm team remove-user <team-id> --user-id <user-id>

# Set team budget
pllm team set-budget <team-id> --amount 10000

# Update team
pllm team update <team-id> --name "New Name" --max-budget 7500
```

### API Key Management

```bash
# Generate API key for user
pllm key generate --type api --user-id <user-id> --name "My API Key" --max-budget 100

# Generate API key for team
pllm key generate --type api --team-id <team-id> --name "Team Key" --duration 2592000

# List API keys
pllm key list

# List keys for specific user
pllm key list --user-id <user-id>

# Get key details
pllm key get <key-id>

# Revoke key
pllm key revoke <key-id> --reason "No longer needed"

# Get key usage info
pllm key info <key-id>
```

### Budget Management

```bash
# Show global budget status
pllm budget status

# Show user budget status
pllm budget status --user-id <user-id>

# Show team budget status
pllm budget status --team-id <team-id>

# Set budget for user
pllm budget set --entity-type user --entity-id <user-id> --amount 2000

# Set budget for team
pllm budget set --entity-type team --entity-id <team-id> --amount 5000

# Reset budget spend
pllm budget reset --entity-type user --entity-id <user-id>

# Show budget usage
pllm budget usage --user-id <user-id> --days 30

# Generate budget report
pllm budget report --period monthly
```

### Configuration Management

```bash
# Show current configuration
pllm config show

# Show configuration in JSON format
pllm config show --json
```

## Command Reference

### Global Flags

- `--config string`: Config file path (default: `~/.pllm.yaml`)
- `--db-url string`: Database URL for direct access
- `--api-url string`: API base URL for remote access
- `--api-key string`: API key for remote access
- `--json`: Output in JSON format
- `--verbose`: Verbose output

### Access Modes

#### Direct Database Access
When running on the server with database access:
```bash
pllm --db-url "postgres://user:pass@localhost:5432/pllm" user list
```

#### Remote API Access
When running remotely:
```bash
pllm --api-url "https://pllm.example.com" --api-key "your-key" user list
```

## Output Formats

### Table Format (Default)
```
ID                                   Email           Role    Active  Created
------------------------------------ --------------- ------- ------- ----------------
12345678-1234-1234-1234-123456789abc user@example.com admin   true    2024-01-15 10:30
```

### JSON Format
```bash
pllm --json user list
```

```json
[
  {
    "id": "12345678-1234-1234-1234-123456789abc",
    "email": "user@example.com",
    "role": "admin",
    "is_active": true,
    "created_at": "2024-01-15T10:30:00Z"
  }
]
```

## Error Handling

The CLI provides clear error messages and appropriate exit codes:

- `0`: Success
- `1`: General error
- `2`: Configuration error
- `3`: Authentication error
- `4`: Not found error

## Security Considerations

- API keys are never displayed in full after creation
- Database credentials should be properly secured
- Use environment variables or secure config files for sensitive data
- Keys are hashed before storage in the database

## Development

### Adding New Commands

1. Create command file in `cmd/pllm/commands/`
2. Implement both database and API access methods
3. Add command to main CLI in `cmd/pllm/main.go`
4. Update this README

### Testing

Test with database access:
```bash
go run ./cmd/pllm --db-url "postgres://..." user list
```

Test with API access:
```bash
go run ./cmd/pllm --api-url "https://..." --api-key "..." user list
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Update documentation
6. Submit a pull request