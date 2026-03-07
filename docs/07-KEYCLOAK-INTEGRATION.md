# KORS Volume 7 : Intégration Keycloak et OAuth2

Ce document détaille la procédure pour authentifier et autoriser un module métier ou un service tiers via le SSO Keycloak.

## 1. Concept d'Authentification de Service

KORS utilise le flux **OAuth2 Client Credentials**. Ce mécanisme permet à un module (ex: `mes-service`) de prouver son identité sans intervention humaine, en utilisant un couple `Client ID` / `Client Secret`.

## 2. Configuration dans Keycloak

Pour chaque nouveau module, suivez ces étapes dans la console d'administration Keycloak :

1.  **Créer un Client** :
    *   Client ID : `nom-du-module` (ex: `tms-service`).
    *   Client Protocol : `openid-connect`.
2.  **Activer le Service Account** :
    *   Capability Config : Activez "Service Accounts Enabled".
    *   Authentication Flow : Désactivez "Standard Flow" (pas besoin de login/pass humain).
3.  **Récupérer les Credentials** :
    *   Allez dans l'onglet "Credentials".
    *   Copiez le "Client Secret".
4.  **Identifier l'ID du Service** :
    *   Keycloak génère un UUID interne pour ce compte de service. Cet identifiant est présent dans le champ `sub` des tokens générés.

## 3. Enregistrement de l'Identité dans KORS

Une fois le client créé dans Keycloak, vous devez déclarer son identité dans KORS pour lui attacher des droits.

### Mutation GraphQL
```graphql
mutation {
  createIdentity(input: {
    externalId: "UUID-KEYCLOAK-DU-SERVICE",
    name: "TMS Production Service",
    type: "service"
  }) {
    id # Renvoie l'UUID interne KORS
  }
}
```

## 4. Attribution des Permissions

Par défaut, le module n'a aucun droit. Vous devez lui accorder les actions nécessaires.

### Exemple : Droit de lecture et d'écriture sur les ressources
```graphql
mutation {
  grantPermission(input: {
    identityId: "UUID-INTERNE-KORS",
    action: "write"
  }) { success }
}
```

## 5. Flux d'utilisation pour le Développeur

Le module métier doit suivre ce cycle pour chaque requête :

### A. Obtention du Token (Backend to Backend)
Le module appelle le endpoint de Keycloak :
*   **URL** : `http://kors-sso:8080/realms/kors/protocol/openid-connect/token`
*   **Method** : `POST`
*   **Body** (x-www-form-urlencoded) :
    *   `grant_type`: `client_credentials`
    *   `client_id`: `tms-service`
    *   `client_secret`: `VOTRE_SECRET`

### B. Appel de l'API KORS
Utilisez le `access_token` reçu dans le header de chaque requête GraphQL :
```http
Authorization: Bearer <TOKEN_JWT>
```

## 6. Sécurité Technique : Validation RS256

L'API KORS (`kors-api`) valide l'authenticité du token via :
1.  **Signature** : Vérification cryptographique avec la clé publique de Keycloak (récupérée via `JWKS_ENDPOINT`).
2.  **Expiration** : Rejet systématique des tokens dont le champ `exp` est dépassé.
3.  **Audience** : Vérification que le token a bien été émis pour le royaume KORS.
