# KORS Volume 5 : Sécurité et Modèle d'Identité

## 1. Authentification SSO (Keycloak)
KORS délègue l'authentification à **Keycloak**.
*   **Protocole** : OIDC (OpenID Connect).
*   **Format** : Token JWT RS256.
*   **Vérification** : `kors-api` télécharge les clés publiques au démarrage via le `JWKS_ENDPOINT`.

## 2. Le Registre des Identités (`kors.identities`)
Chaque utilisateur Keycloak possède un `external_id` (le `sub` du JWT). 
Au premier appel, KORS enregistre cet utilisateur dans son registre local pour pouvoir lui attacher des permissions métier.

## 3. Système RBAC (Permissions)
KORS utilise un modèle de permissions restrictif. Par défaut, rien n'est autorisé.

### Niveaux de portée (Scope)
Une permission peut être accordée à trois niveaux :
1.  **Global** : L'identité peut agir sur n'importe quel objet (ex: droit `admin` pour provisionner).
2.  **Par Type** : L'identité peut agir sur toutes les ressources d'un certain type (ex: `write` sur toutes les `cnc_machine`).
3.  **Par Ressource** : L'identité peut agir sur un UUID spécifique uniquement.

### Actions Standards
*   `read` : Consultation simple.
*   `write` : Création de nouvelles ressources.
*   `transition` : Capacité à déclencher un changement d'état.
*   `admin` : Gestion des métadonnées système et des droits.

## 4. Expiration des Droits
Toute permission peut posséder une date `expires_at`. Le service `kors-worker` purge automatiquement les droits périmés toutes les heures.
