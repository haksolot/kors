# Référence Technique API KORS (v1)

L'API KORS est le point d'entrée unique pour la gestion du cycle de vie des ressources.

## 1. Informations de Connexion

| Élément | Valeur en Développement | Valeur en Production |
|---|---|---|
| **Endpoint GraphQL** | `http://localhost:8080/query` | `https://api.kors.safran-ls.com/query` |
| **Endpoint WebSocket** | `ws://localhost:8080/query` | `wss://api.kors.safran-ls.com/query` |
| **Protocole Sub** | `graphql-ws` | `graphql-ws` |
| **Headers requis** | `Content-Type: application/json` | `Content-Type: application/json` |
| **Authentification** | `Authorization: Bearer <JWT>` | `Authorization: Bearer <JWT>` |

---

## 2. Opérations Fondamentales

### A. Enregistrer un ResourceType (`registerResourceType`)
**Utilité** : Définit le contrat technique d'un module métier.
**Droits requis** : Action `admin` au niveau global.

**Requête :**
```graphql
mutation {
  registerResourceType(input: {
    name: "cnc_machine",
    description: "Machine à commande numérique",
    jsonSchema: {
      type: "object",
      required: ["serial"],
      properties: {
        serial: { type: "string" },
        firmware: { type: "string" }
      }
    },
    transitions: {
      "OFFLINE": ["ONLINE"],
      "ONLINE": ["OFFLINE", "PRODUCING", "MAINTENANCE"],
      "PRODUCING": ["ONLINE", "OFFLINE"],
      "MAINTENANCE": ["OFFLINE", "ONLINE"]
    }
  }) {
    success
    resourceType { id name createdAt }
    error { code message }
  }
}
```

### B. Créer une Resource (`createResource`)
**Utilité** : Instancie une entité. Génère l'UUID universel.
**Droits requis** : Action `write` sur le `ResourceType`.

**Requête :**
```graphql
mutation {
  createResource(input: {
    typeName: "cnc_machine",
    initialState: "OFFLINE",
    metadata: { serial: "CNC-FR-001", firmware: "v2.4.1" }
  }) {
    success
    resource { 
      id 
      state 
      createdAt 
    }
    error { message }
  }
}
```

### C. Transition d'état (`transitionResource`)
**Utilité** : Change l'état. Déclenche un événement NATS.
**Droits requis** : Action `transition` sur la `Resource` ou le `ResourceType`.

**Requête :**
```graphql
mutation {
  transitionResource(input: {
    resourceId: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
    toState: "ONLINE",
    metadata: { operator_id: "USER_123" }
  }) {
    success
    resource { state metadata updatedAt }
    error { code message }
  }
}
```

---

## 3. Gestion des Erreurs (MutationError)

L'API KORS ne renvoie pas d'erreurs GraphQL standard pour les erreurs métier. Elle utilise le champ `error` dans le résultat.

| Code d'erreur | Signification |
|---|---|
| `REGISTRATION_FAILED` | Le type existe déjà ou le schéma est invalide. |
| `CREATION_FAILED` | Le type n'existe pas ou l'identité n'a pas le droit `write`. |
| `TRANSITION_FAILED` | La transition n'est pas autorisée par la machine à états ou manque de droit `transition`. |
| `FORBIDDEN` | L'identité n'a aucun droit sur l'objet. |
| `INVALID_STATE` | L'état initial n'appartient pas au graphe défini. |
