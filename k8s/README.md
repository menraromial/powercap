# Kubernetes Deployment Guide

Ce guide explique comment déployer et configurer le Power Manager sur Kubernetes avec différents providers de données.

## 🚀 Déploiement rapide

### 1. Déploiement avec EPEX (défaut)
```bash
# Appliquer la configuration RBAC
kubectl apply -f k8s/rbac.yaml

# Déployer avec la configuration EPEX par défaut
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/daemonset.yaml
```

### 2. Vérifier le déploiement
```bash
# Vérifier les pods
kubectl get pods -l app=powercap-manager

# Vérifier les logs
kubectl logs -l app=powercap-manager

# Vérifier les labels des nœuds
kubectl get nodes -o custom-columns=NAME:.metadata.name,RAPL_MAX:.metadata.labels.rapl/max_power_uw,RAPL_CURRENT:.metadata.labels.rapl/pmax
```

## ⚙️ Configuration des Providers

### EPEX Provider (Production)

**ConfigMap pour le marché français :**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-config-epex-fr
data:
  DATA_PROVIDER: "epex"
  PROVIDER_URL: "https://www.epexspot.com/en/market-results"
  PROVIDER_PARAMS: |
    {
      "market_area": "FR",
      "auction": "IDA1",
      "modality": "Auction",
      "sub_modality": "Intraday",
      "data_mode": "table"
    }
```

**ConfigMap pour le marché allemand :**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-config-epex-de
data:
  DATA_PROVIDER: "epex"
  PROVIDER_URL: "https://www.epexspot.com/en/market-results"
  PROVIDER_PARAMS: |
    {
      "market_area": "DE",
      "auction": "IDA1",
      "modality": "Auction",
      "sub_modality": "Intraday",
      "data_mode": "table"
    }
```

### Mock Provider (Développement/Test)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-config-mock
data:
  DATA_PROVIDER: "mock"
  PROVIDER_URL: ""
  PROVIDER_PARAMS: "{}"
```

### Static Provider (Démonstration)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-config-static
data:
  DATA_PROVIDER: "static"
  PROVIDER_URL: ""
  PROVIDER_PARAMS: "{}"
```

## 🔧 Déploiement avec un Provider Spécifique

### 1. Utiliser un ConfigMap existant
```bash
# Déployer avec le provider Mock
kubectl patch daemonset powercap-manager -p '{"spec":{"template":{"spec":{"containers":[{"name":"powercap-manager","envFrom":[{"configMapRef":{"name":"powercap-config-mock"}}]}]}}}}'

# Déployer avec le marché allemand
kubectl patch daemonset powercap-manager -p '{"spec":{"template":{"spec":{"containers":[{"name":"powercap-manager","envFrom":[{"configMapRef":{"name":"powercap-config-epex-de"}}]}]}}}}'
```

### 2. Créer une configuration personnalisée
```bash
# Créer un ConfigMap personnalisé
kubectl create configmap powercap-config-custom \
  --from-literal=DATA_PROVIDER=epex \
  --from-literal=PROVIDER_URL=https://your-custom-epex-url.com \
  --from-literal='PROVIDER_PARAMS={"market_area":"BE","auction":"IDA1","modality":"Auction","sub_modality":"Intraday","data_mode":"table"}'

# Utiliser la configuration personnalisée
kubectl patch daemonset powercap-manager -p '{"spec":{"template":{"spec":{"containers":[{"name":"powercap-manager","envFrom":[{"configMapRef":{"name":"powercap-config-custom"}}]}]}}}}'
```

## 📊 Configuration des Paramètres de Puissance

### Variables d'environnement principales

| Variable | Description | Défaut | Exemple |
|----------|-------------|---------|---------|
| `DATA_PROVIDER` | Type de provider | `epex` | `epex`, `mock`, `static` |
| `PROVIDER_URL` | URL du provider | EPEX URL | `https://api.custom-provider.com` |
| `PROVIDER_PARAMS` | Paramètres JSON | EPEX FR | `{"market_area":"DE"}` |
| `MAX_SOURCE` | Puissance max (µW) | `40000000` | `35000000` |
| `STABILISATION_TIME` | Intervalle (s) | `300` | `600` |
| `RAPL_MIN_POWER` | Puissance min (µW) | `10000000` | `8000000` |

### Exemple de configuration complète
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-config-production
data:
  # Gestion de puissance
  MAX_SOURCE: "50000000"
  STABILISATION_TIME: "180"
  RAPL_MIN_POWER: "12000000"
  
  # Provider EPEX pour l'Italie
  DATA_PROVIDER: "epex"
  PROVIDER_URL: "https://www.epexspot.com/en/market-results"
  PROVIDER_PARAMS: |
    {
      "market_area": "IT",
      "auction": "IDA1",
      "modality": "Auction",
      "sub_modality": "Intraday",
      "data_mode": "table"
    }
  
  # Rafraîchissement toutes les 4 heures
  DATA_REFRESH_CRON: "0 */4 * * *"
```

## 🔍 Monitoring et Observabilité

### Logs importants
```bash
# Logs de démarrage
kubectl logs -l app=powercap-manager | grep "Starting professional power management"

# Logs de configuration du provider
kubectl logs -l app=powercap-manager | grep "Configured data provider"

# Logs de calcul de puissance
kubectl logs -l app=powercap-manager | grep "Power calculation"

# Logs d'erreur
kubectl logs -l app=powercap-manager | grep -i error
```

### Métriques sur les nœuds
```bash
# Vérifier les labels RAPL
kubectl get nodes -o json | jq '.items[].metadata.labels | with_entries(select(.key | startswith("rapl/")))'

# Vérifier les annotations d'initialisation
kubectl get nodes -o json | jq '.items[].metadata.annotations | with_entries(select(.key | contains("power-manager")))'
```

## 🛠️ Dépannage

### Provider non reconnu
```bash
# Vérifier la configuration
kubectl describe configmap powercap-config

# Logs d'erreur de provider
kubectl logs -l app=powercap-manager | grep "unknown provider type"
```

### Problème de connexion EPEX
```bash
# Vérifier la connectivité réseau
kubectl exec -it $(kubectl get pods -l app=powercap-manager -o jsonpath='{.items[0].metadata.name}') -- curl -I https://www.epexspot.com

# Logs de scraping
kubectl logs -l app=powercap-manager | grep "HTTP request"
```

### Problème d'accès RAPL
```bash
# Vérifier les permissions
kubectl exec -it $(kubectl get pods -l app=powercap-manager -o jsonpath='{.items[0].metadata.name}') -- ls -la /sys/devices/virtual/powercap/

# Vérifier les privilèges
kubectl get pods -l app=powercap-manager -o yaml | grep -A 5 securityContext
```

## 🔄 Mise à jour de Configuration

### Changement de provider à chaud
```bash
# 1. Créer nouvelle configuration
kubectl create configmap powercap-config-new --from-literal=DATA_PROVIDER=mock

# 2. Mettre à jour le DaemonSet
kubectl patch daemonset powercap-manager -p '{"spec":{"template":{"spec":{"containers":[{"name":"powercap-manager","envFrom":[{"configMapRef":{"name":"powercap-config-new"}}]}]}}}}'

# 3. Vérifier le rollout
kubectl rollout status daemonset/powercap-manager
```

### Rollback en cas de problème
```bash
# Revenir à la configuration précédente
kubectl rollout undo daemonset/powercap-manager

# Vérifier le statut
kubectl rollout status daemonset/powercap-manager
```

## 📋 Exemples de Déploiement

### Environnement de développement
```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-dev
data:
  DATA_PROVIDER: "mock"
  MAX_SOURCE: "30000000"
  STABILISATION_TIME: "60"
  PROVIDER_PARAMS: "{}"
EOF

kubectl patch daemonset powercap-manager -p '{"spec":{"template":{"spec":{"containers":[{"name":"powercap-manager","envFrom":[{"configMapRef":{"name":"powercap-dev"}}]}]}}}}'
```

### Environnement de production
```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-prod
data:
  DATA_PROVIDER: "epex"
  PROVIDER_URL: "https://www.epexspot.com/en/market-results"
  MAX_SOURCE: "60000000"
  STABILISATION_TIME: "300"
  PROVIDER_PARAMS: '{"market_area":"FR","auction":"IDA1","modality":"Auction","sub_modality":"Intraday","data_mode":"table"}'
EOF

kubectl patch daemonset powercap-manager -p '{"spec":{"template":{"spec":{"containers":[{"name":"powercap-manager","envFrom":[{"configMapRef":{"name":"powercap-prod"}}]}]}}}}'
```

Cette approche permet aux administrateurs Kubernetes de facilement changer de provider et de configurer les URLs/paramètres via des ConfigMaps, sans avoir besoin de rebuilder l'image Docker.