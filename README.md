# KORS — Kernel for Operations & Resource Systems

KORS est une plateforme open-source industrielle conçue pour centraliser la traçabilité (EN9100) et le pilotage du cycle de vie des ressources techniques.

Il sert de noyau technique ("Kernel") sur lequel viennent se greffer des modules métier (TMS, MES, PLM) via une architecture fédérée.

## Fonctionnalités Clés

*   **Six Primitives Fondamentales** : Identités, Types de Ressources, Ressources, Événements, Permissions (RBAC) et Révisions.
*   **Architecture Fédérée** : Subgraph compatible Apollo Federation v2 pour un écosystème modulaire.
*   **Traçabilité Immuable** : Journalisation systématique sur bus de données NATS JetStream (Raft consensus).
*   **Stockage Souverain** : Gestion native des snapshots et fichiers binaires via MinIO (S3).
*   **SDKs Professionnels** : Bibliothèques prêtes à l'emploi pour Go, TypeScript et Python.
*   **Résilience Industrielle** : Stateless API, Graceful Shutdown et Verrouillage Distribué.

## Structure du Projet

*   `kors-api/` : Le cœur du système (GraphQL API).
*   `kors-events/` : Relais temps réel et consommateur d'événements.
*   `kors-worker/` : Service de maintenance et tâches de fond.
*   `sdk/` : Bibliothèques clientes générées pour Go, TS et Python.
*   `examples/` : Exemple concret d'un module métier intégré.
*   `docs/` : Documentation technique complète en plusieurs volumes.

## Stack Technique

*   **Backend** : Go 1.25, gqlgen, Chi, pgx.
*   **Infrastructure** : PostgreSQL 15, NATS JetStream, MinIO, Keycloak, Jaeger (OTel).
*   **Orchestration** : Docker Compose & Kubernetes Ready.

## Documentation

Consultez le dossier `/docs` pour approfondir :
1.  [Architecture & Philosophie](./docs/01-CORE-CONCEPTS.md)
2.  [Infrastructure & Opérations](./docs/02-INFRASTRUCTURE-OPERATIONS.md)
3.  [Spécification API & SDK](./docs/03-API-SPECIFICATION.md)
4.  [Intégration de Modules](./docs/04-MODULE-INTEGRATION.md)
5.  [Modèle de Sécurité](./docs/05-SECURITY-MODEL.md)

## Licence

Ce projet est distribué sous licence MIT. Voir le fichier `LICENSE` pour plus de détails.
