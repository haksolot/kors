# KORS Volume 3 : Spécification API et SDK

Ce document est la référence technique pour interagir avec le noyau KORS.

## 1. Endpoints et Protocoles

| Service | Protocole | URL (Développement) |
|---|---|---|
| GraphQL Core | HTTP POST | `http://localhost/query` |
| Subscriptions | WebSockets | `ws://localhost/query` |
| Playground UI | HTTP GET | `http://localhost/` |

**Authentification** : Toutes les requêtes (sauf introspection en dev) exigent un header `Authorization: Bearer <token>`.

## 2. Primitives de Gouvernance (Admin uniquement)

### Lister les modules actifs (`provisionedModules`)
Permet de voir quels services métier sont actuellement branchés sur KORS.
```graphql
query {
  provisionedModules # Renvoie ["tms", "mes", ...]
}
```

### Provisionner un module (`provisionModule`)
Crée un bac à sable (schéma SQL + utilisateur) pour un nouveau service.
```graphql
mutation {
  provisionModule(moduleName: "nom_du_module") {
    success
    username # user_nom_du_module
    password # Mot de passe généré
    schema   # nom_du_module
  }
}
```

### Supprimer un module (`deprovisionModule`)
Nettoie intégralement la base de données (données métier et accès).
```graphql
mutation {
  deprovisionModule(moduleName: "nom_du_module")
}
```

## 3. Primitives de Ressources

### Enregistrer un type (`registerResourceType`)
Définit le contrat technique d'une entité.
*   **jsonSchema** : Schéma de validation pour les métadonnées.
*   **transitions** : Graphe des états autorisés (ex: `idle` -> `in_use`).

### Créer une ressource (`createResource`)
Génère une enveloppe KORS avec un UUID unique.
*   L'opération est atomique : si la notification NATS échoue, la ressource n'est pas créée.

## 4. Utilisation des SDKs

Les SDKs sont générés automatiquement et garantissent le typage des réponses.
*   **Go** : `github.com/haksolot/kors/sdk/go`
*   **TS** : `@kors/sdk` (via npm)
*   **Python** : `kors-sdk` (via pip)
