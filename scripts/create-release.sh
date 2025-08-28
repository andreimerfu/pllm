#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print usage
usage() {
    echo "Usage: $0 [patch|minor|major|prerelease] [prerelease-type] [custom-version]"
    echo ""
    echo "Examples:"
    echo "  $0 patch                    # v1.0.0 -> v1.0.1"
    echo "  $0 minor                    # v1.0.1 -> v1.1.0"
    echo "  $0 major                    # v1.1.0 -> v2.0.0"
    echo "  $0 prerelease rc            # v1.0.0 -> v1.0.1-rc1"
    echo "  $0 custom v2.1.0            # Direct version specification"
    echo "  $0 custom v2.1.0-beta2      # Custom prerelease version"
    echo ""
    echo "Prerelease types: rc, alpha, beta"
    exit 1
}

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo -e "${RED}Error: Not in a git repository${NC}"
    exit 1
fi

# Check if we have uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
    echo -e "${YELLOW}Warning: You have uncommitted changes.${NC}"
    echo -e "${YELLOW}It's recommended to commit or stash them before creating a release.${NC}"
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Default values
VERSION_TYPE=${1:-patch}
PRERELEASE_TYPE=${2:-rc}
CUSTOM_VERSION=${3:-}

# Validate arguments
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    usage
fi

if [ "$VERSION_TYPE" = "custom" ] && [ -z "$CUSTOM_VERSION" ]; then
    CUSTOM_VERSION=$PRERELEASE_TYPE
fi

# Get current version
echo -e "${BLUE}Getting current version...${NC}"
LATEST_TAG=$(git tag -l --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+' | head -1)
if [ -z "$LATEST_TAG" ]; then
    LATEST_TAG="v0.0.0"
    echo -e "${YELLOW}No previous version found, starting from v0.0.0${NC}"
else
    echo -e "${GREEN}Current version: $LATEST_TAG${NC}"
fi

# Calculate new version
if [ -n "$CUSTOM_VERSION" ]; then
    NEW_VERSION="$CUSTOM_VERSION"
    # Ensure it starts with 'v'
    if [[ ! "$NEW_VERSION" =~ ^v ]]; then
        NEW_VERSION="v$NEW_VERSION"
    fi
    echo -e "${BLUE}Using custom version: $NEW_VERSION${NC}"
else
    # Parse current version
    CURRENT_VERSION=${LATEST_TAG#v}
    IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"
    
    # Handle pre-release versions
    if [[ "$PATCH" =~ ^([0-9]+)-.* ]]; then
        PATCH="${BASH_REMATCH[1]}"
    fi
    
    case "$VERSION_TYPE" in
        patch)
            PATCH=$((PATCH + 1))
            NEW_VERSION="v${MAJOR}.${MINOR}.${PATCH}"
            ;;
        minor)
            MINOR=$((MINOR + 1))
            PATCH=0
            NEW_VERSION="v${MAJOR}.${MINOR}.${PATCH}"
            ;;
        major)
            MAJOR=$((MAJOR + 1))
            MINOR=0
            PATCH=0
            NEW_VERSION="v${MAJOR}.${MINOR}.${PATCH}"
            ;;
        prerelease)
            # Find existing prerelease number or start at 1
            EXISTING_PRE=$(git tag -l --sort=-version:refname | grep -E "^v${MAJOR}\.${MINOR}\.${PATCH}-${PRERELEASE_TYPE}[0-9]+" | head -1)
            if [ -n "$EXISTING_PRE" ]; then
                PRE_NUM=$(echo "$EXISTING_PRE" | sed -E "s/.*-${PRERELEASE_TYPE}([0-9]+).*/\1/")
                PRE_NUM=$((PRE_NUM + 1))
            else
                PRE_NUM=1
            fi
            NEW_VERSION="v${MAJOR}.${MINOR}.${PATCH}-${PRERELEASE_TYPE}${PRE_NUM}"
            ;;
        *)
            echo -e "${RED}Error: Invalid version type '$VERSION_TYPE'${NC}"
            usage
            ;;
    esac
    
    echo -e "${BLUE}Calculated new version: $NEW_VERSION${NC}"
fi

# Check if version already exists
if git tag -l | grep -q "^${NEW_VERSION}$"; then
    echo -e "${RED}Error: Version $NEW_VERSION already exists!${NC}"
    exit 1
fi

# Show what will happen
echo -e "${YELLOW}Summary:${NC}"
echo -e "  Current version: ${LATEST_TAG}"
echo -e "  New version:     ${NEW_VERSION}"
echo -e "  Version type:    ${VERSION_TYPE}"
if [ "$VERSION_TYPE" = "prerelease" ]; then
    echo -e "  Prerelease type: ${PRERELEASE_TYPE}"
fi
echo ""

# Confirm
read -p "Create this release? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Release cancelled${NC}"
    exit 0
fi

# Check if we have the GitHub CLI
if ! command -v gh &> /dev/null; then
    echo -e "${YELLOW}GitHub CLI not found. Manual steps:${NC}"
    echo -e "1. Create and push tag: ${GREEN}git tag -a $NEW_VERSION -m 'Release $NEW_VERSION' && git push origin $NEW_VERSION${NC}"
    echo -e "2. Go to GitHub Actions to trigger the manual release workflow"
    echo -e "3. Or use GitHub web interface: Actions -> Manual Release -> Run workflow"
    exit 0
fi

echo -e "${BLUE}Creating release using GitHub CLI...${NC}"

# Create the tag locally first
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION"
git push origin "$NEW_VERSION"

echo -e "${GREEN}✅ Tag $NEW_VERSION created and pushed${NC}"
echo -e "${BLUE}🚀 GitHub Actions will now build and publish the release${NC}"
echo -e "${BLUE}📋 Check progress: https://github.com/amerfu/pllm/actions${NC}"

# Show the release URL once it's created
echo -e "${GREEN}📦 Release will be available at: https://github.com/amerfu/pllm/releases/tag/$NEW_VERSION${NC}"