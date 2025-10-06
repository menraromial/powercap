# PowerCap Manager - Docker Image Tagging & Release Guide

## 🏷️ Image Tagging Strategy

The PowerCap Manager uses a dual tagging strategy for Docker images:

### **Automatic Tags**
- **`latest`**: Always points to the latest main branch build
- **`v0.0.4`**: Version-specific tags for releases
- **`main-abc1234`**: Branch and commit SHA for traceability

### **Registry Location**
```
ghcr.io/menraromial/powercap:latest
ghcr.io/menraromial/powercap:v0.0.4
ghcr.io/menraromial/powercap:main-abc1234
```

## 🚀 Creating Releases

### **Quick Release (Recommended)**
```bash
# Auto-increment patch version and push
./scripts/tag-version.sh --patch --push

# Auto-increment minor version
./scripts/tag-version.sh --minor --push

# Specific version
./scripts/tag-version.sh 0.0.5 --push
```

### **Manual Process**
```bash
# 1. Create and push version tag
git tag -a v0.0.5 -m "Release v0.0.5"
git push origin v0.0.5

# 2. GitHub Actions automatically:
#    - Builds Docker image
#    - Tags with v0.0.5 and latest
#    - Pushes to ghcr.io
```

## 🔄 GitHub Actions Workflow

### **Triggers**
The workflow runs on:
- **Main branch pushes**: Creates `latest` tag
- **Version tags**: Creates version-specific tags (e.g., `v0.0.4`)

### **Workflow File: `.github/workflows/build-push.yml`**
```yaml
on:
  push:
    branches: [ "main" ]
    tags: [ "v*.*.*" ]  # Triggers on v0.0.4, v1.2.3, etc.

jobs:
  build-and-push:
    steps:
      - name: Extract Docker metadata
        uses: docker/metadata-action@v5
        with:
          images: ghcr.io/${{ github.repository }}
          tags: |
            # Latest for main branch
            type=raw,value=latest,enable={{is_default_branch}}
            # Version for tags
            type=ref,event=tag
            # SHA for traceability
            type=sha,prefix={{branch}}-
```

### **Generated Tags Example**
For tag `v0.0.5`, the workflow creates:
```
ghcr.io/menraromial/powercap:v0.0.5    # Version tag
ghcr.io/menraromial/powercap:latest    # Latest tag
```

## 🛠️ Using the Tagging Script

### **Installation**
```bash
# Script is already executable
./scripts/tag-version.sh --help
```

### **Basic Usage**
```bash
# Create version v0.0.5
./scripts/tag-version.sh 0.0.5

# Create and push to trigger build
./scripts/tag-version.sh 0.0.5 --push

# Preview what would happen
./scripts/tag-version.sh 0.0.5 --dry-run
```

### **Automatic Versioning**
```bash
# Increment patch version (0.0.4 → 0.0.5)
./scripts/tag-version.sh --patch --push

# Increment minor version (0.0.4 → 0.1.0)
./scripts/tag-version.sh --minor --push

# Increment major version (0.0.4 → 1.0.0)
./scripts/tag-version.sh --major --push
```

### **Advanced Options**
```bash
# Force overwrite existing tag
./scripts/tag-version.sh 0.0.4 --force --push

# Create locally without pushing
./scripts/tag-version.sh 0.0.6

# Later push the tag
git push origin v0.0.6
```

## 📦 Using Released Images

### **Latest Development Version**
```yaml
# Kubernetes DaemonSet
spec:
  containers:
  - name: powercap-manager
    image: ghcr.io/menraromial/powercap:latest
```

### **Specific Version (Recommended for Production)**
```yaml
# Kubernetes DaemonSet
spec:
  containers:
  - name: powercap-manager
    image: ghcr.io/menraromial/powercap:v0.0.5
```

### **Docker Run**
```bash
# Latest version
docker run --privileged ghcr.io/menraromial/powercap:latest

# Specific version
docker run --privileged ghcr.io/menraromial/powercap:v0.0.5
```

## 📊 Release Workflow Example

### **Complete Release Process**
```bash
# 1. Make your changes and commit
git add .
git commit -m "feat: Add new power calculation algorithm"
git push origin main

# 2. Create and push version tag
./scripts/tag-version.sh --patch --push

# 3. Monitor GitHub Actions
# Visit: https://github.com/menraromial/powercap/actions

# 4. Verify images are available
docker pull ghcr.io/menraromial/powercap:v0.0.5
docker pull ghcr.io/menraromial/powercap:latest
```

### **Hotfix Release**
```bash
# 1. Create hotfix branch
git checkout -b hotfix/critical-fix

# 2. Make fix and test
git commit -m "fix: Critical RAPL calculation error"
git push origin hotfix/critical-fix

# 3. Merge to main
git checkout main
git merge hotfix/critical-fix
git push origin main

# 4. Create emergency release
./scripts/tag-version.sh --patch --push
```

## 🔍 Monitoring Releases

### **GitHub Actions Status**
```bash
# Check workflow status
gh run list --workflow=build-push.yml

# View specific run
gh run view <run-id>
```

### **Image Verification**
```bash
# List available tags
docker search ghcr.io/menraromial/powercap

# Pull and inspect specific version
docker pull ghcr.io/menraromial/powercap:v0.0.5
docker inspect ghcr.io/menraromial/powercap:v0.0.5
```

### **Container Registry**
Visit: https://github.com/menraromial/powercap/pkgs/container/powercap

## 📚 Version Numbering Guidelines

### **Semantic Versioning (SemVer)**
```
v<MAJOR>.<MINOR>.<PATCH>

MAJOR: Breaking changes (e.g., v1.0.0)
MINOR: New features, backward compatible (e.g., v0.1.0)  
PATCH: Bug fixes, backward compatible (e.g., v0.0.5)
```

### **Release Types**
```bash
# Bug fixes
./scripts/tag-version.sh --patch --push     # v0.0.4 → v0.0.5

# New features
./scripts/tag-version.sh --minor --push     # v0.0.5 → v0.1.0

# Breaking changes
./scripts/tag-version.sh --major --push     # v0.1.0 → v1.0.0
```

## 🚨 Troubleshooting

### **Build Failures**
```bash
# Check GitHub Actions logs
gh run view --log

# Test build locally
docker build -t test-powercap .
```

### **Tag Conflicts**
```bash
# Force overwrite existing tag
./scripts/tag-version.sh 0.0.5 --force --push

# Delete problematic tag
git tag -d v0.0.5
git push origin :refs/tags/v0.0.5
```

### **Registry Issues**
```bash
# Re-authenticate to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin

# Manually push image
docker build -t ghcr.io/menraromial/powercap:v0.0.5 .
docker push ghcr.io/menraromial/powercap:v0.0.5
```

This system provides automated, reliable Docker image building and tagging for the PowerCap Manager with full version control integration.