# KORS Volume 4 : Intégration de Modules et Fédération

Ce document explique comment brancher un nouveau service métier sur l'écosystème KORS de manière totalement automatisée.

## 1. Automatisation du Provisionnement

KORS élimine le besoin d'administration manuelle de la base de données. Chaque module dispose d'un bac à sable (Sandbox) sécurisé.

### Flux d'activation :
1.  **Demande** : Le développeur (ou la CI/CD) appelle `provisionModule(moduleName: "nom")`.
2.  **Exécution KORS** : 
    *   Création du schéma `nom`.
    *   Création de l'utilisateur `user_nom`.
    *   Génération d'un mot de passe fort.
    *   Configuration des privilèges de sécurité.
3.  **Réponse** : Le module reçoit ses credentials.

## 2. Étanchéité et Privilèges SQL

L'architecture multi-schémas garantit que les erreurs d'un module n'impactent pas le cœur du système.

| Périmètre | Droits de l'utilisateur du module | Usage |
|---|---|---|
| **Schéma `kors`** | `SELECT` uniquement | Lecture de l'index universel et des événements. |
| **Schéma métier** | `ALL PRIVILEGES` (Owner) | Gestion autonome des tables métier. |
| **Autres schémas** | Aucun accès | Isolation totale entre les modules. |

## 3. Gestion Automatisée des Tables (Auto-Migration)

Les développeurs ne doivent jamais modifier la structure de la base de données manuellement. Chaque module est responsable de son propre schéma métier.

### Recommandation technique :
Le module doit embarquer un moteur de migration (ex: `goose`, `migrate`) qui s'exécute au démarrage (Init container ou au début du `main`).

```go
// Exemple de création de table métier autonome
func ensureSchema(db *pgxpool.Pool) {
    query := `CREATE TABLE IF NOT EXISTS tms.tools (
        id UUID PRIMARY KEY,
        serial_number TEXT NOT NULL
    );`
    db.Exec(context.Background(), query)
}
```

## 4. Cycle de Vie d'une Entité Fédérée

1.  **Enregistrement** : Le module déclare son `ResourceType` via KORS API.
2.  **Création** : Le module demande la création d'une `Resource` à KORS, reçoit un UUID.
3.  **Stockage** : Le module stocke ses données métier dans ses propres tables en utilisant cet UUID comme clé primaire.
4.  **Extension** : Le module expose un Subgraph GraphQL qui enrichit l'objet `Resource` de KORS avec ses propres champs.
