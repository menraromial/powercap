#!/bin/bash
# PowerCap Manager - Version Tagging Script
# This script helps create version tags and trigger Docker image builds

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

print_info() {
    print_status "$BLUE" "ℹ️  $1"
}

print_success() {
    print_status "$GREEN" "✅ $1"
}

print_warning() {
    print_status "$YELLOW" "⚠️  $1"
}

print_error() {
    print_status "$RED" "❌ $1"
}

# Function to show help
show_help() {
    cat << EOF
PowerCap Manager - Version Tagging Script

USAGE:
    $0 <version> [OPTIONS]

ARGUMENTS:
    version         Version to tag (e.g., 0.0.4, 1.2.3)
                   Will be prefixed with 'v' automatically

OPTIONS:
    --major         Increment major version (x.0.0)
    --minor         Increment minor version (x.y.0)  
    --patch         Increment patch version (x.y.z)
    --dry-run       Show what would be done without executing
    --force         Force tag creation (overwrite existing)
    --push          Also push tags to remote
    -h, --help      Show this help message

EXAMPLES:
    $0 0.0.4                    # Create tag v0.0.4
    $0 0.0.4 --push             # Create and push tag v0.0.4
    $0 --patch                  # Auto-increment patch version
    $0 --minor --push           # Auto-increment minor and push
    $0 1.0.0 --dry-run          # Preview tag creation

WORKFLOW INTEGRATION:
    This script works with the GitHub Actions workflow to automatically:
    1. Build Docker images with version tags
    2. Push to GitHub Container Registry (ghcr.io)
    3. Tag images with both 'latest' and version (e.g., 'v0.0.4')

EOF
}

# Function to get current version from git tags
get_current_version() {
    local current_tag
    current_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    echo "${current_tag#v}"  # Remove 'v' prefix
}

# Function to increment version
increment_version() {
    local version="$1"
    local part="$2"
    
    IFS='.' read -r -a version_parts <<< "$version"
    local major="${version_parts[0]:-0}"
    local minor="${version_parts[1]:-0}"
    local patch="${version_parts[2]:-0}"
    
    case "$part" in
        "major")
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        "minor")
            minor=$((minor + 1))
            patch=0
            ;;
        "patch")
            patch=$((patch + 1))
            ;;
        *)
            print_error "Invalid version part: $part"
            exit 1
            ;;
    esac
    
    echo "$major.$minor.$patch"
}

# Function to validate version format
validate_version() {
    local version="$1"
    if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        print_error "Invalid version format: $version"
        print_info "Expected format: x.y.z (e.g., 0.0.4, 1.2.3)"
        exit 1
    fi
}

# Function to check if tag exists
tag_exists() {
    local tag="$1"
    git tag -l | grep -q "^${tag}$"
}

# Function to create git tag
create_tag() {
    local version="$1"
    local tag="v${version}"
    local dry_run="$2"
    local force="$3"
    local push="$4"
    
    # Check if tag already exists
    if tag_exists "$tag" && [[ "$force" != "true" ]]; then
        print_error "Tag $tag already exists"
        print_info "Use --force to overwrite or choose a different version"
        exit 1
    fi
    
    # Show what will be done
    print_info "Creating tag: $tag"
    
    if [[ "$dry_run" == "true" ]]; then
        print_info "[DRY-RUN] Would execute: git tag ${force:+--force} -a $tag -m \"Release $tag\""
        if [[ "$push" == "true" ]]; then
            print_info "[DRY-RUN] Would execute: git push origin $tag"
        fi
        return 0
    fi
    
    # Create the tag
    local tag_cmd="git tag"
    if [[ "$force" == "true" ]]; then
        tag_cmd="$tag_cmd --force"
    fi
    tag_cmd="$tag_cmd -a $tag -m \"Release $tag\""
    
    if eval "$tag_cmd"; then
        print_success "Created tag: $tag"
    else
        print_error "Failed to create tag: $tag"
        exit 1
    fi
    
    # Push tag if requested
    if [[ "$push" == "true" ]]; then
        local push_cmd="git push origin $tag"
        if [[ "$force" == "true" ]]; then
            push_cmd="git push --force origin $tag"
        fi
        
        if eval "$push_cmd"; then
            print_success "Pushed tag to origin: $tag"
            print_info "GitHub Actions will now build Docker image with tags:"
            print_info "  - ghcr.io/menraromial/powercap:$tag"
            print_info "  - ghcr.io/menraromial/powercap:latest"
        else
            print_error "Failed to push tag: $tag"
            exit 1
        fi
    fi
}

# Function to show tag preview
show_preview() {
    local version="$1"
    local tag="v${version}"
    
    print_info "Tag Preview:"
    echo "=============="
    echo "Tag:           $tag"
    echo "Docker Images: ghcr.io/menraromial/powercap:$tag"
    echo "               ghcr.io/menraromial/powercap:latest"
    echo "Trigger:       GitHub Actions build-push workflow"
    echo ""
}

# Parse command line arguments
VERSION=""
INCREMENT=""
DRY_RUN=false
FORCE=false
PUSH=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --major)
            INCREMENT="major"
            shift
            ;;
        --minor)
            INCREMENT="minor"
            shift
            ;;
        --patch)
            INCREMENT="patch"
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --force)
            FORCE=true
            shift
            ;;
        --push)
            PUSH=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        -*)
            print_error "Unknown option: $1"
            show_help
            exit 1
            ;;
        *)
            if [[ -z "$VERSION" ]]; then
                VERSION="$1"
            else
                print_error "Multiple versions specified"
                show_help
                exit 1
            fi
            shift
            ;;
    esac
done

# Main execution
main() {
    print_info "PowerCap Manager - Version Tagging"
    print_info "=================================="
    
    # Check if we're in a git repository
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        print_error "Not in a git repository"
        exit 1
    fi
    
    # Determine version
    if [[ -n "$INCREMENT" ]]; then
        if [[ -n "$VERSION" ]]; then
            print_error "Cannot specify both version and increment option"
            exit 1
        fi
        
        local current_version
        current_version=$(get_current_version)
        VERSION=$(increment_version "$current_version" "$INCREMENT")
        print_info "Current version: v$current_version"
        print_info "New version: v$VERSION"
    elif [[ -z "$VERSION" ]]; then
        print_error "Version or increment option required"
        show_help
        exit 1
    fi
    
    # Validate version
    validate_version "$VERSION"
    
    # Show preview
    show_preview "$VERSION"
    
    # Create tag
    create_tag "$VERSION" "$DRY_RUN" "$FORCE" "$PUSH"
    
    if [[ "$DRY_RUN" != "true" ]]; then
        print_success "Version tagging completed!"
        
        if [[ "$PUSH" == "true" ]]; then
            print_info "Monitor the GitHub Actions workflow at:"
            print_info "https://github.com/menraromial/powercap/actions"
        else
            print_warning "Tag created locally. Use --push to trigger GitHub Actions build"
        fi
    fi
}

# Run main function
main "$@"