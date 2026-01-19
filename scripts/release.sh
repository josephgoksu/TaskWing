#!/bin/bash
#
# TaskWing Release Script
# Usage: make release
#
# This script handles the release process:
# 1. Verifies clean git state
# 2. Runs tests
# 3. Prompts for version bump type
# 4. Opens editor for release notes
# 5. Creates annotated tag and pushes to trigger CI/CD
#
# Note: Version is derived from git tags at build time (ldflags).
# No source files are modified during release.
#
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored message
info() { echo -e "${BLUE}$1${NC}"; }
success() { echo -e "${GREEN}$1${NC}"; }
warn() { echo -e "${YELLOW}$1${NC}"; }
error() { echo -e "${RED}$1${NC}" >&2; }

# Check for required commands
check_requirements() {
    info "Checking requirements..."

    if ! command -v git &> /dev/null; then
        error "git is required but not installed."
        exit 1
    fi

    if ! command -v go &> /dev/null; then
        error "go is required but not installed."
        exit 1
    fi

    success "Requirements met."
}

# Ensure working directory is clean
check_git_clean() {
    info "Checking git status..."

    if [ -n "$(git status --porcelain)" ]; then
        error "Working directory is not clean. Commit or stash changes first."
        git status --short
        exit 1
    fi

    success "Working directory is clean."
}

# Run tests
run_tests() {
    info "Running tests..."

    if ! make test; then
        error "Tests failed. Fix them before releasing."
        exit 1
    fi

    success "All tests passed."
}

# Get current version from git tags
get_current_version() {
    local version
    version=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    echo "$version"
}

# Parse version into components
parse_version() {
    local version=$1
    version=${version#v}  # Remove 'v' prefix

    local major minor patch
    IFS='.' read -r major minor patch <<< "$version"

    echo "$major $minor $patch"
}

# Calculate next version based on bump type
calc_next_version() {
    local current=$1
    local bump_type=$2

    read -r major minor patch <<< "$(parse_version "$current")"

    case $bump_type in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
    esac

    echo "v${major}.${minor}.${patch}"
}

# Validate semver format
validate_semver() {
    local version=$1
    if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        error "Invalid version format: $version (expected vX.Y.Z)"
        exit 1
    fi
}

# Prompt user for version bump type
prompt_version() {
    local current=$1

    echo ""
    info "Current version: $current"
    echo ""
    echo "Select release type:"
    echo "  1) Patch  $(calc_next_version "$current" patch)"
    echo "  2) Minor  $(calc_next_version "$current" minor)"
    echo "  3) Major  $(calc_next_version "$current" major)"
    echo "  4) Custom"
    echo ""

    read -rp "Choice [1-4]: " choice

    case $choice in
        1) echo "$(calc_next_version "$current" patch)" ;;
        2) echo "$(calc_next_version "$current" minor)" ;;
        3) echo "$(calc_next_version "$current" major)" ;;
        4)
            read -rp "Enter custom version (vX.Y.Z): " custom
            validate_semver "$custom"
            echo "$custom"
            ;;
        *)
            error "Invalid choice"
            exit 1
            ;;
    esac
}

# Get release notes from user via editor
get_release_notes() {
    local version=$1
    local tmpfile
    tmpfile=$(mktemp)

    # Pre-populate with template
    cat > "$tmpfile" << EOF
# Release Notes for $version
#
# Write your release notes below.
# Lines starting with # will be removed.
#
# Example:
# ## What's New
# - Feature 1
# - Feature 2
#
# ## Bug Fixes
# - Fix 1
#

EOF

    # Open editor
    local editor="${EDITOR:-vim}"
    $editor "$tmpfile"

    # Remove comments and empty lines at start
    local notes
    notes=$(grep -v '^#' "$tmpfile" | sed '/./,$!d')

    if [ -z "$notes" ]; then
        error "Release notes cannot be empty."
        rm "$tmpfile"
        exit 1
    fi

    echo "$notes" > "$tmpfile"
    echo "$tmpfile"
}

# Create and push tag
create_and_push_tag() {
    local version=$1
    local notes_file=$2

    info "Creating tag $version..."
    git tag -a "$version" -F "$notes_file"

    echo ""
    warn "Ready to push $version to origin."
    read -rp "Push now? [y/N]: " confirm

    if [[ $confirm =~ ^[Yy]$ ]]; then
        info "Pushing tag to origin..."
        git push origin "$version"
        success "Released $version!"
        echo ""
        info "GitHub Actions will now build and publish the release."
    else
        warn "Tag created locally but not pushed."
        warn "To push later: git push origin $version"
    fi

    # Cleanup
    rm -f "$notes_file"
}

# Main
main() {
    echo ""
    info "TaskWing Release Script"
    info "========================"
    echo ""

    check_requirements
    check_git_clean
    run_tests

    current_version=$(get_current_version)
    new_version=$(prompt_version "$current_version")

    validate_semver "$new_version"

    echo ""
    info "Preparing release $new_version..."

    notes_file=$(get_release_notes "$new_version")

    create_and_push_tag "$new_version" "$notes_file"

    echo ""
    success "Done!"
}

main "$@"
