# KORS Volume 8 : Gestion des Migrations de Schéma

Ce document explique comment faire évoluer la structure de la base de données (Cœur et Modules) sans perte de données.

## 1. Principes de Migration

La modification directe et manuelle de la base de données en production est **interdite**. Toutes les modifications doivent être :
*   **Versionnées** : Un script SQL par changement.
*   **Automatisées** : Appliquées par un outil de migration.
*   **Idempotentes** : Capables d'être relancées sans erreur (ex: `ADD COLUMN IF NOT EXISTS`).

## 2. Évolution du Cœur KORS (Schéma `kors`)

Les migrations du cœur sont situées dans `kors-api/migrations/`.
Elles sont appliquées via la commande :
```bash
make migrate
```
PostgreSQL garantit l'atomicité du changement : si le script échoue au milieu, la base revient à son état précédent (Rollback).

## 3. Évolution des Modules Métier

Chaque module est propriétaire de son propre schéma (ex: `tms`). Pour le faire évoluer :

### A. Méthode Automatique (Recommandée)
Le module doit intégrer ses propres scripts de migration dans son code source. Au démarrage du service, celui-ci vérifie et applique les changements manquants.

**Exemple avec l'outil Goose (Go) :**
1.  Créez un dossier `migrations/`.
2.  Ajoutez un fichier `00001_votre_changement.sql`.
3.  Lancer la migration dans le `main.go` avant de démarrer le serveur :
```go
import "github.com/pressly/goose/v3"

func main() {
    // ... connexion DB
    if err := goose.Up(db, "migrations"); err != nil {
        log.Fatal(err)
    }
}
```

### B. Pourquoi cette autonomie ?
*   **Indépendance** : Vous pouvez déployer une nouvelle version de votre module avec un nouveau schéma sans attendre que l'équipe KORS intervienne.
*   **Sécurité** : KORS a déjà provisionné votre utilisateur avec les droits `ALL PRIVILEGES` sur votre schéma, vous permettant de faire des `ALTER TABLE` librement.

## 4. Bonnes Pratiques pour la Production

1.  **Colonnes avec valeurs par défaut** : Si vous ajoutez une colonne `NOT NULL`, donnez-lui une valeur par défaut pour ne pas bloquer les lignes existantes.
2.  **Changements Non-Destructifs** : Préférez ajouter des colonnes plutôt que de renommer les anciennes (pour éviter de casser les versions précédentes du module qui tourneraient encore).
3.  **Tests de Rollback** : Toujours écrire la partie `-- +goose Down` pour pouvoir revenir en arrière en cas de problème.
