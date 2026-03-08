# KORS Volume 4 : Intégration de Modules et Gouvernance

Ce document explique comment gérer le cycle de vie d'un module métier sur l'écosystème KORS de manière totalement automatisée et sécurisée.

## 1. Cycle de Vie d'un Module

La gouvernance des modules s'effectue via l'API GraphQL de KORS (accès réservé au rôle `admin`).

### A. Provisionnement (`provisionModule`)
Crée un environnement isolé pour un nouveau module.
*   **Action** : `mutation { provisionModule(moduleName: "tms") { ... } }`
*   **Opérations système** :
    *   Création d'un schéma PostgreSQL `tms`.
    *   Création d'un utilisateur `user_tms` avec mot de passe aléatoire.
    *   Attribution des droits de lecture seule sur le cœur KORS.
    *   Transfert de propriété du schéma `tms` à `user_tms`.

### B. Inventaire (`provisionedModules`)
Permet de lister tous les modules actuellement connectés à la plateforme.
*   **Action** : `query { provisionedModules }`
*   **Usage** : Audit de l'écosystème et vérification des déploiements.

### C. Déprovisionnement (`deprovisionModule`)
Supprime proprement un module et ses accès.
*   **Action** : `mutation { deprovisionModule(moduleName: "tms") }`
*   **Sécurité** : Cette opération est atomique. Elle réassigne les objets, révoque les privilèges et supprime le rôle et le schéma (`CASCADE`) pour garantir qu'aucune trace résiduelle ne pollue la base de données.

## 2. Étanchéité et Privilèges SQL

L'architecture multi-schémas garantit que les erreurs d'un module n'impactent pas le cœur du système.

| Périmètre | Droits de l'utilisateur du module | Usage |
|---|---|---|
| **Schéma `kors`** | `SELECT` uniquement | Lecture de l'index universel et des événements. |
| **Schéma métier** | `ALL PRIVILEGES` (Owner) | Gestion autonome des tables métier. |
| **Autres schémas** | Aucun accès | Isolation totale entre les modules. |

## 3. Gestion Automatisée des Tables (Auto-Migration)

Le développeur du module est responsable de la structure de son schéma. Le module doit embarquer son propre moteur de migration (ex: `goose`, `migrate`) qui s'exécute au démarrage.

```go
// Exemple de création de table métier autonome au démarrage du module
func ensureSchema(db *pgxpool.Pool) {
    query := `CREATE TABLE IF NOT EXISTS tms.tools (
        id UUID PRIMARY KEY,
        serial_number TEXT NOT NULL
    );`
    db.Exec(context.Background(), query)
}
```

## 4. Pattern de l'Entité Fédérée

Pour chaque objet métier nécessitant une traçabilité :
1.  **Création technique** : Le module appelle `createResource` sur KORS API et reçoit un UUID.
2.  **Stockage métier** : Le module enregistre ses données dans son schéma local en utilisant cet UUID comme clé primaire.
3.  **Extension GraphQL** : Le module expose un Subgraph qui "étend" le type `Resource` de KORS pour y ajouter ses champs spécifiques.
