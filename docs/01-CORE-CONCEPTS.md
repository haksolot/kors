# KORS Volume 1 : Architecture et Philosophie

## 1. Raison d'être
KORS (Kernel for Operations & Resource Systems) est le socle technique commun de The KORS Project. Son but est de fournir une infrastructure de confiance pour la traçabilité (EN9100) et le pilotage du cycle de vie des entités industrielles.

## 2. Le Modèle "Enveloppe vs Contenu" (Symbiose)
KORS introduit une séparation stricte des responsabilités :

### A. L'Enveloppe KORS (Le Contenant)
Gérée exclusivement par l'API KORS dans le schéma `kors`. Elle garantit :
*   **Identité** : UUID universel.
*   **État** : Position dans la machine à états finis (FSM).
*   **Audit** : Historique complet de chaque modification.
*   **Droits** : Permissions d'accès et d'action.

### B. Le Contenu Métier (Le Contenu)
Géré par chaque module métier (ex: TMS, MES) dans son schéma PostgreSQL dédié (ex: `tms`). Il contient :
*   **Données Riches** : Dimensions, quantités, spécifications techniques.
*   **Logique Métier** : Calculs spécifiques au domaine.

**Point de pivot** : L'UUID. L'ID KORS est la clé primaire dans la table métier.

## 3. Les Six Primitives du Noyau
Toute la valeur de KORS repose sur ces 6 briques :
1.  **Identities** : Registre universel des acteurs (utilisateurs, services, machines).
2.  **ResourceTypes** : Contrats techniques définissant le JSON Schema et le graphe de transitions autorisées.
3.  **Resources** : L'instance d'une entité avec son état courant.
4.  **Events** : Le journal immuable et le bus de diffusion temps réel.
5.  **Permissions** : Système RBAC hiérarchique (Global > Type > Resource).
6.  **Revisions** : Snapshots versionnés liés à des fichiers physiques (S3/MinIO).

## 4. Architecture Distribuée
Le système est conçu comme un **réseau uniforme** :
*   **Stateless API** : Plusieurs instances de `kors-api` derrière un Load Balancer.
*   **Consensus Bus** : Cluster NATS JetStream (Algorithme Raft).
*   **High Availability DB** : Cluster PostgreSQL avec Patroni.
*   **Sovereign Storage** : Cluster MinIO avec Erasure Coding.
