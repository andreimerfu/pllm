# GitHub Deployment Configuration

This document outlines the required GitHub secrets, variables, and configuration needed for automated CI/CD pipelines.

## Required GitHub Secrets

Configure these secrets in your GitHub repository settings (`Settings > Secrets and variables > Actions`):

### Docker Hub Configuration (Required)

| Secret Name | Description | Example |
|-------------|-------------|---------|
| `DOCKERHUB_USERNAME` | Docker Hub username | `amerfu` |
| `DOCKERHUB_TOKEN` | Docker Hub access token | `dckr_pat_xyz123...` |

**Setup Instructions:**
1. Go to [Docker Hub Account Settings](https://hub.docker.com/settings/security)
2. Click "New Access Token"
3. Name: `PLLM GitHub Actions`
4. Permissions: `Public Repo Read/Write`
5. Copy the generated token to `DOCKERHUB_TOKEN` secret

### Optional Integrations

| Secret Name | Description | Required For | Example |
|-------------|-------------|--------------|---------|
| `CODECOV_TOKEN` | Codecov upload token | Code coverage reports | `abc123-def456-...` |
| `ARTIFACTHUB_TOKEN` | ArtifactHub API token | Publishing to ArtifactHub | `ah_xyz123...` |
| `SLACK_WEBHOOK_URL` | Slack webhook for notifications | Release notifications | `https://hooks.slack.com/...` |

## GitHub Token Permissions

The default `GITHUB_TOKEN` needs these permissions (configured automatically):

- `contents: write` - For creating releases and pushing to gh-pages
- `packages: write` - For publishing OCI artifacts
- `pages: write` - For GitHub Pages deployment
- `id-token: write` - For OIDC authentication

## Repository Settings

### Branch Protection

Configure branch protection rules for `main` branch:

```yaml
# Recommended settings
require_status_checks: true
required_status_checks:
  - "test-go"
  - "test-frontend"
  - "test-helm"
  # - "security-scan"
dismiss_stale_reviews: true
require_code_owner_reviews: true
required_approving_review_count: 1
```

### GitHub Pages

Enable GitHub Pages for Helm chart repository:

1. Go to `Settings > Pages`
2. Source: `Deploy from a branch`
3. Branch: `gh-pages`
4. Folder: `/root`

## Workflow Triggers

### Automated Triggers

| Workflow | Trigger | Description |
|----------|---------|-------------|
| `ci.yml` | Push to `main`/`develop`, PRs | Run tests and security scans |
| `docker-build.yml` | Push to `main`/`develop`, tags `v*` | Build and push Docker images |
| `helm-release.yml` | Tags `v*` | Release Helm charts |
| `release.yml` | Tags `v*` | Create GitHub releases |

### Manual Triggers

Some workflows support manual triggering:

```bash
# Trigger Helm release manually
gh workflow run helm-release.yml

# Trigger full release process
gh workflow run release.yml
```

## Release Process

### Automatic Release (Recommended)

1. **Create Release Tag:**
   ```bash
   git tag v1.2.3
   git push origin v1.2.3
   ```

2. **Automated Steps:**
   - Creates GitHub release with changelog
   - Builds and pushes Docker images (`amerfu/pllm:1.2.3`)
   - Packages and publishes Helm chart
   - Updates chart repository index
   - Publishes to ArtifactHub (if configured)
   - Sends notifications (if configured)

### Manual Release

If you need to release manually:

```bash
# 1. Build and push Docker image
docker buildx build --platform linux/amd64,linux/arm64 \
  -t amerfu/pllm:v1.2.3 \
  -t amerfu/pllm:latest \
  --push .

# 2. Package and push Helm chart
cd deploy/helm
helm dependency update pllm
helm package pllm
helm push pllm-1.2.3.tgz oci://registry-1.docker.io/amerfu

# 3. Create GitHub release
gh release create v1.2.3 \
  --title "Release v1.2.3" \
  --notes "See CHANGELOG.md for details" \
  pllm-1.2.3.tgz
```

## Configuration Examples

### Complete Secrets Setup

```bash
# Required secrets
gh secret set DOCKERHUB_USERNAME -b"amerfu"
gh secret set DOCKERHUB_TOKEN -b"dckr_pat_xyz123..."

# Optional secrets
gh secret set CODECOV_TOKEN -b"abc123-def456-..."
gh secret set ARTIFACTHUB_TOKEN -b"ah_xyz123..."
gh secret set SLACK_WEBHOOK_URL -b"https://hooks.slack.com/services/..."
```

### Environment Variables

For local testing, create `.env` file:

```bash
# Docker Hub
DOCKERHUB_USERNAME=amerfu
DOCKERHUB_TOKEN=dckr_pat_xyz123...

# Optional integrations
CODECOV_TOKEN=abc123-def456-...
ARTIFACTHUB_TOKEN=ah_xyz123...
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...
```

## Troubleshooting

### Common Issues

1. **Docker Hub Push Fails**
   - Verify `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` are correct
   - Ensure token has `Public Repo Read/Write` permissions
   - Check repository exists: `amerfu/pllm`

2. **Helm Chart Release Fails**
   - Verify chart dependencies are available
   - Check Helm chart syntax: `helm lint deploy/helm/pllm`
   - Ensure OCI registry permissions

3. **GitHub Pages Not Working**
   - Enable GitHub Pages in repository settings
   - Verify `gh-pages` branch exists
   - Check workflow permissions

### Debug Commands

```bash
# Test Docker Hub authentication
echo $DOCKERHUB_TOKEN | docker login -u $DOCKERHUB_USERNAME --password-stdin

# Test Helm chart
cd deploy/helm
helm dependency update pllm
helm lint pllm
helm template test-release pllm

# Test GitHub CLI authentication
gh auth status
gh release list
```

## Security Considerations

### Secrets Management

- **Never commit secrets to code**
- Use GitHub secrets for sensitive data
- Rotate tokens regularly (every 90 days)
- Use least-privilege access tokens

### Workflow Security

- Pin action versions with SHA hashes
- Use official actions when possible
- Review third-party actions before use
- Enable Dependabot for workflow dependencies

### Example Secure Workflow

```yaml
# Pin to specific SHA instead of tag
- uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1

# Use official actions
- uses: docker/setup-buildx-action@v3

# Validate inputs
- name: Validate version
  run: |
    if [[ ! "${{ github.ref_name }}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      echo "Invalid version format"
      exit 1
    fi
```

## Monitoring

### Workflow Status

Monitor deployment status:

```bash
# Check recent workflow runs
gh run list --limit 10

# View specific workflow
gh run view 1234567890

# Watch workflow in real-time
gh run watch 1234567890
```

### Metrics and Alerting

Set up monitoring for:

- Build success/failure rates
- Deployment frequency
- Lead time for changes
- Mean time to recovery

### Notifications

Configure Slack notifications for:

```yaml
# In workflow file
- name: Notify on failure
  if: failure()
  uses: 8398a7/action-slack@v3
  with:
    status: failure
    text: "Deployment failed for ${{ github.ref_name }}"
  env:
    SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}
```

## Next Steps

1. **Configure all required secrets**
2. **Test the CI/CD pipeline with a test release**
3. **Set up monitoring and alerting**
4. **Document your specific deployment procedures**
5. **Train team members on the release process**

## Support

For issues with the deployment pipeline:

1. Check workflow logs in GitHub Actions
2. Review this documentation
3. Open an issue in the repository
4. Contact the maintainers
