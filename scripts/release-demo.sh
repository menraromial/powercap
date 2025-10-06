#!/bin/bash
# PowerCap Manager - Release Example Script
# This demonstrates the complete release workflow

set -euo pipefail

echo "🚀 PowerCap Manager - Release Workflow Demo"
echo "============================================"
echo ""

echo "📋 Current Git Status:"
echo "Branch: $(git branch --show-current)"
echo "Last commit: $(git log --oneline -1)"
echo "Current tags: $(git tag -l | tail -3 | tr '\n' ' ')"
echo ""

echo "🏷️ Version Tagging Options:"
echo ""

echo "1️⃣  Auto-increment patch version:"
echo "   ./scripts/tag-version.sh --patch --push"
echo "   Result: Creates next patch version (e.g., v0.0.4 → v0.0.5)"
echo ""

echo "2️⃣  Specific version:"
echo "   ./scripts/tag-version.sh 1.0.0 --push"
echo "   Result: Creates v1.0.0 tag"
echo ""

echo "3️⃣  Preview mode (safe):"
echo "   ./scripts/tag-version.sh --patch --dry-run"
echo "   Result: Shows what would happen without executing"
echo ""

echo "🔄 What happens after pushing a tag:"
echo "1. GitHub Actions detects new tag (v*.*.)"
echo "2. Builds Docker image from source"
echo "3. Tags image with both version and 'latest'"
echo "4. Pushes to ghcr.io/menraromial/powercap"
echo ""

echo "📦 Resulting Docker Images:"
echo "ghcr.io/menraromial/powercap:v0.0.5    # Version-specific"
echo "ghcr.io/menraromial/powercap:latest    # Always latest"
echo ""

echo "💡 Usage in Kubernetes:"
echo "# Production (pinned version)"
echo "image: ghcr.io/menraromial/powercap:v0.0.5"
echo ""
echo "# Development (latest)"
echo "image: ghcr.io/menraromial/powercap:latest"
echo ""

echo "🛠️  Try it now:"
echo "   ./scripts/tag-version.sh --patch --dry-run"
echo ""