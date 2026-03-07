# Guide du Développeur KORS

Ce guide explique comment intégrer un module métier (ex: TMS, MES, PLM) au noyau technique **KORS** (Kernel for Operations & Resource Systems).

---

## 1. Modèle Mental : Enveloppe vs Contenu

KORS ne contient pas la logique métier. Il fournit une **enveloppe technique** (la `Resource`) qui garantit quatre propriétés fondamentales :
*   **Identité** : Un UUID universel partagé entre KORS et votre module.
*   **État** : Une machine à états contrôlée et inviolable.
*   **Traçabilité** : Un journal d'événements immuable (conforme EN9100).
*   **Droits** : Un système de permissions (RBAC) granulaire.

**Règle d'or** : Votre module gère les données riches (ex: dimensions d'un outil, quantité d'une commande) dans son propre schéma PostgreSQL. KORS gère l'enveloppe dans le schéma `kors`.

---

## 2. Architecture de Communication

L'interaction avec KORS se fait via deux canaux principaux :

### A. Flux Synchrone : API GraphQL
Utilisé pour les actions immédiates et la lecture de l'index universel.
*   **Endpoint** : `http://kors-api:8080/query`
*   **Actions** : Créer une ressource, effectuer une transition, uploader un plan (Révision).

### B. Flux Asynchrone : Bus NATS JetStream
Utilisé pour le découplage entre les modules.
*   **Sujets** : `kors.resource.created`, `kors.resource.state_changed`, etc.
*   **Principe** : Votre module publie ou écoute des événements sans jamais appeler un autre module directement.

---

## 3. Guide d'Intégration (Pas à Pas)

### Étape 1 : Déclarer son contrat (ResourceType)
Avant de créer des objets, vous devez dire à KORS ce qu'est votre entité.
*   Envoyez une mutation `registerResourceType`.
*   Définissez votre **JSON Schema** (pour valider les métadonnées).
*   Définissez vos **Transitions** (quels états sont autorisés après quels états).

### Étape 2 : Créer une Resource
Quand vous créez un objet métier (ex: un nouvel outil), créez simultanément son enveloppe dans KORS.
*   KORS vous renvoie un UUID.
*   **Important** : Stockez cet UUID comme clé primaire (ou étrangère) dans votre propre base de données.

### Étape 3 : Piloter le Cycle de Vie
Pour changer l'état d'un objet (ex: `disponible` -> `en maintenance`) :
*   Appelez `transitionResource`.
*   KORS vérifie :
    1.  Si vous avez le droit (Permissions).
    2.  Si le passage entre ces deux états est autorisé pour ce type.
*   Si c'est validé, KORS enregistre l'état et publie un événement sur NATS.

---

## 4. Sécurité & Authentification

### Authentification
Toutes les requêtes vers `kors-api` doivent inclure un token JWT valide émis par le Keycloak Safran.
*   Header : `Authorization: Bearer <JWT>`
*   L'identité est automatiquement extraite du champ `sub` du token.

### Permissions (RBAC)
Le système est "fermé" par défaut. Pour agir, une identité doit posséder une permission dans `kors.permissions` :
*   `read` : Lire l'index.
*   `write` : Créer des ressources d'un certain type.
*   `transition` : Faire changer l'état d'une ressource.
*   `admin` : Gérer les types et les droits.

---

## 5. Stockage des fichiers (Révisions)

Si votre module génère des fichiers critiques (ex: un rapport de contrôle, un plan de maintenance) :
*   Utilisez la mutation `createRevision`.
*   Envoyez le contenu du fichier (Base64) et les métadonnées actuelles.
*   KORS stocke le fichier dans MinIO, prend un "instantané" de l'objet et journalise l'action.

---

## 6. Configuration du Module

Pour se brancher sur KORS en production, votre module a besoin de :
*   `KORS_API_URL` : URL du serveur GraphQL.
*   `NATS_URL` : Adresse du bus pour écouter/publier.
*   `KEYCLOAK_URL` : Pour obtenir les tokens d'accès.

---

## 7. Exemple de Flux complet (Dashboard Temps Réel)

1.  Le client s'abonne en WebSocket via `subscription { eventWasPublished { ... } }`.
2.  Un opérateur déplace un outil -> Mutation `transitionResource`.
3.  L'API valide et publie sur NATS.
4.  Le service `kors-events` reçoit le message et le pousse au client WebSocket.
5.  Le Dashboard se met à jour instantanément sans aucun rafraîchissement.
