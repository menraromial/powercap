# 🎯 Configuration Kubernetes Complétée

## ✅ Mission Accomplie

Le Power Manager est maintenant **entièrement configurable via Kubernetes** et permet aux administrateurs de changer de provider et de configurer les URLs/paramètres via des fichiers YAML.

## 🚀 Nouvelles Fonctionnalités Kubernetes

### 1. **Configuration Centralisée via ConfigMaps**

```yaml
# Exemple de ConfigMap pour EPEX France
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
      "sub_modality": "Intraday"
    }
```

### 2. **Providers Configurables**

| Provider | Usage | Configuration |
|----------|-------|---------------|
| **EPEX** | Production | URLs et paramètres de marché configurables |
| **Mock** | Développement | Données synthétiques, pas de réseau |
| **Static** | Démonstration | Données prédéfinies |

### 3. **Factory Pattern pour l'Extensibilité**

```go
// Création automatique du provider selon la config
factory := providers.NewProviderFactory()
provider, err := factory.CreateProvider(cfg)
```

### 4. **Validation de Configuration**

- ✅ Validation des types de providers
- ✅ Validation des paramètres requis
- ✅ Validation des URLs
- ✅ Messages d'erreur explicites

### 5. **Architecture Complète Kubernetes**

```
k8s/
├── configmap.yaml     # Configurations pour différents providers
├── daemonset.yaml     # Déploiement sur tous les nœuds
├── rbac.yaml          # Permissions Kubernetes
├── kustomization.yaml # Gestion des environnements
├── .env.example       # Exemple de variables
└── README.md          # Guide complet de déploiement
```

## 🔧 Configuration Administrative

### **Changer de Provider**
```bash
# Basculer vers Mock pour les tests
kubectl patch daemonset powercap-manager -p '{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "powercap-manager",
          "envFrom": [{"configMapRef": {"name": "powercap-config-mock"}}]
        }]
      }
    }
  }
}'
```

### **Configurer un Nouveau Marché**
```bash
# Créer config pour marché allemand
kubectl create configmap powercap-config-epex-de \
  --from-literal=DATA_PROVIDER=epex \
  --from-literal=PROVIDER_URL=https://www.epexspot.com/en/market-results \
  --from-literal='PROVIDER_PARAMS={"market_area":"DE","auction":"IDA1"}'
```

### **URLs Personnalisées**
```yaml
# ConfigMap avec URL personnalisée
data:
  DATA_PROVIDER: "epex"
  PROVIDER_URL: "https://your-custom-epex-mirror.com/api"
  PROVIDER_PARAMS: '{"custom_param":"value"}'
```

## 📊 Variables d'Environnement Disponibles

### **Configuration du Provider**
- `DATA_PROVIDER` : Type de provider (`epex`, `mock`, `static`)
- `PROVIDER_URL` : URL de base du provider
- `PROVIDER_PARAMS` : Paramètres JSON du provider
- `DATA_REFRESH_CRON` : Expression cron pour la synchronisation

### **Configuration de Puissance**
- `MAX_SOURCE` : Puissance source maximum (µW)
- `STABILISATION_TIME` : Intervalle d'ajustement (secondes)
- `RAPL_MIN_POWER` : Puissance minimum RAPL (µW)
- `NODE_NAME` : Nom du nœud (automatique via Kubernetes)

## 🎯 Valeurs par Défaut Intelligentes

```json
{
  "defaults": {
    "DATA_PROVIDER": "epex",
    "PROVIDER_URL": "https://www.epexspot.com/en/market-results",
    "PROVIDER_PARAMS": {
      "market_area": "FR",
      "auction": "IDA1",
      "modality": "Auction",
      "sub_modality": "Intraday",
      "data_mode": "table"
    },
    "MAX_SOURCE": "40000000",
    "STABILISATION_TIME": "300"
  }
}
```

## 🔄 Workflow Administrateur

### 1. **Déploiement Initial**
```bash
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/daemonset.yaml
```

### 2. **Changement de Configuration**
```bash
# Modifier le ConfigMap
kubectl edit configmap powercap-config

# Redémarrer les pods pour appliquer les changements
kubectl rollout restart daemonset/powercap-manager
```

### 3. **Monitoring**
```bash
# Vérifier les logs
kubectl logs -l app=powercap-manager

# Vérifier les labels des nœuds
kubectl get nodes -o custom-columns=NAME:.metadata.name,RAPL:.metadata.labels.rapl/pmax
```

## 🚀 Exemples de Déploiement

### **Environnement de Production (Multi-pays)**
```bash
# France
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-config-fr
data:
  DATA_PROVIDER: "epex"
  PROVIDER_PARAMS: '{"market_area":"FR","auction":"IDA1"}'
EOF

# Allemagne  
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-config-de
data:
  DATA_PROVIDER: "epex"
  PROVIDER_PARAMS: '{"market_area":"DE","auction":"IDA1"}'
EOF
```

### **Environnement de Test**
```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-config-test
data:
  DATA_PROVIDER: "mock"
  STABILISATION_TIME: "60"
  PROVIDER_PARAMS: "{}"
EOF
```

## 🎉 Résultats

### ✅ **Configuration Kubernetes Native**
- ConfigMaps pour tous les paramètres
- DaemonSet avec permissions RBAC
- Support multi-environnements avec Kustomize

### ✅ **Flexibilité Administrative**  
- Changement de provider sans rebuild
- URLs et paramètres configurables à chaud
- Validation automatique des configurations

### ✅ **Production Ready**
- Dockerfile optimisé multi-stage
- Health checks intégrés
- Gestion gracieuse des erreurs

### ✅ **Documentation Complète**
- Guide de déploiement détaillé
- Exemples pour tous les cas d'usage
- Procédures de dépannage

Le système permet maintenant aux **administrateurs Kubernetes de configurer entièrement le Power Manager via des fichiers YAML**, avec support complet pour différents providers, URLs personnalisées, et paramètres de marché, le tout sans avoir besoin de modifier ou rebuilder le code.