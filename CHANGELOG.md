# Modifications apportées au Power Manager

## Résumé des changements

Le système de gestion de l'énergie a été amélioré avec une intégration complète des données du marché EPEX pour un calcul de puissance basé sur les conditions réelles du marché énergétique.

## 🔧 Modifications techniques

### 1. Imports ajoutés
- `"io"` : Pour la lecture des réponses HTTP
- `"net/http"` : Pour les requêtes web vers EPEX
- `"regexp"` : Pour l'extraction de données depuis le HTML

### 2. Nouvelles structures
```go
// MarketData pour les données brutes du scraping
type MarketData struct {
    Period string
    Volume string  
    Price  string
}
```

### 3. Fonctions de scraping ajoutées
- `generateEpexData()` : Point d'entrée pour la génération de données
- `scrapeEPEXData()` : Scraping des données depuis le site EPEX
- `extractPeriods()` : Extraction des périodes temporelles
- `extractTableData()` / `extractTableDataAlternative()` : Extraction des volumes et prix
- `saveToCSV()` : Sauvegarde au format CSV
- `minInt()` : Fonction utilitaire

### 4. Gestion dynamique des fichiers CSV
- Nom de fichier basé sur la date : `epex_data_YYYY-MM-DD.csv`
- Génération automatique si le fichier n'existe pas
- Fallback vers les données de la veille si nécessaire

### 5. Nouveau calcul de puissance
**Avant** (sinusoïdal) :
```go
power := MaxSource * math.Pow(math.Sin((math.Pi/16)*(t-4)), Alpha)
```

**Après** (basé sur EPEX) :
```go
power := (currentVolume / maxVolume) * MaxSource
```

### 6. Planification automatique
- `scheduleDailyEpexGeneration()` : Calcule le temps jusqu'à minuit
- Goroutine pour exécution à minuit
- Rechargement automatique des données en mémoire

### 7. Mode test ajouté
```bash
./powercap test-epex  # Test manuel de génération EPEX
```

## 📊 Fonctionnement du nouveau système

### Cycle quotidien
1. **00:00** : Génération automatique des données EPEX du jour
2. **Continu** : Calcul de puissance basé sur la période de 15 minutes courante
3. **Logs** : Suivi détaillé des calculs et opérations

### Calcul de puissance en temps réel
1. Détermine la période de 15 minutes actuelle (ex: "14:30-14:45")
2. Trouve le volume EPEX correspondant dans les données
3. Calcule le volume maximum de la journée
4. Applique la règle de trois : `(volume_actuel / volume_max) × MAX_SOURCE`
5. Retourne la puissance en µW

### Gestion des erreurs
- Absence de données EPEX → génération automatique
- Échec de scraping → utilisation des données de la veille
- Période introuvable → retour à 0 avec log
- Erreurs réseau → logs détaillés pour debugging

## 🚀 Avantages

1. **Données réelles** : Utilise les vraies conditions du marché énergétique
2. **Automatisation** : Génération quotidienne sans intervention manuelle
3. **Robustesse** : Multiples fallbacks en cas d'erreur
4. **Traçabilité** : Logs détaillés pour le monitoring
5. **Flexibilité** : Facile d'adapter à d'autres sources de données
6. **Performance** : Calculs optimisés avec données en mémoire

## 📁 Fichiers modifiés

- `main.go` : Code principal avec intégration EPEX complète
- `README.md` : Documentation mise à jour
- Nouveaux fichiers générés : `epex_data_YYYY-MM-DD.csv`

## 🧪 Tests effectués

- ✅ Compilation sans erreurs
- ✅ Génération manuelle de données EPEX (`./powercap test-epex`)
- ✅ Validation du format CSV (97 lignes = header + 96 périodes)
- ✅ Vérification de la structure des données
- ✅ Test du calcul de puissance avec données réelles

Le système est maintenant prêt pour un déploiement en production avec une intégration complète des données du marché EPEX.