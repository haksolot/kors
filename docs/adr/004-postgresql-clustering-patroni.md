# ADR-004 : Stratégie de Haute Disponibilité PostgreSQL avec Patroni

*   **Statut** : Proposé
*   **Date** : 2026-03-07
*   **Auteur** : Gemini CLI

## 1. Contexte et Problème

KORS est le noyau technique critique de Safran Landing Systems. Toute indisponibilité de la base de données PostgreSQL entraîne l'arrêt des services de traçabilité et de cycle de vie, impactant directement la production industrielle (EN9100). Une instance unique de base de données (Single Point of Failure) n'est pas acceptable pour l'environnement de production.

## 2. Décision

Nous avons décidé d'utiliser **Patroni** pour gérer le clustering et la haute disponibilité de PostgreSQL.

### Architecture cible sur Kubernetes :
1.  **Cluster Patroni** : Déploiement de 3 instances PostgreSQL (1 Leader, 2 Replicas).
2.  **DCS (Distributed Configuration Store)** : Utilisation de **etcd** comme cerveau du cluster pour la gestion du consensus et l'élection du Leader.
3.  **Accès au Cluster** : Utilisation d'un service Kubernetes de type `ClusterIP` pointant vers un ensemble de pods étiquetés comme `master` par Patroni.
4.  **Replication** : Utilisation de la réplication en streaming synchrone/asynchrone native de PostgreSQL pilotée par Patroni.

## 3. Justification

*   **Failover Automatique** : Patroni détecte la chute du Leader et promeut un Replica en quelques secondes.
*   **Consistance** : L'intégration avec etcd garantit qu'il n'y aura jamais deux Leaders en même temps (Split-brain protection).
*   **Standard Industriel** : Patroni est largement éprouvé chez Safran et dans le monde Cloud Native.
*   **Maintenance facilitée** : Les mises à jour de version de Postgres peuvent être faites en "rolling update" sans interruption de service majeure.

## 4. Conséquences

*   **Complexité accrue** : Nécessite la gestion d'un cluster etcd en plus du cluster Postgres.
*   **Latence de Réplication** : Un léger surcoût réseau pour la synchronisation des données entre les nœuds.
*   **Ressources** : Consommation CPU/RAM multipliée par 3 pour la base de données.

## 5. Alternatives considérées

*   **PostgreSQL simple** : Rejeté pour manque de résilience.
*   **Repmgr** : Rejeté car moins adapté aux environnements Kubernetes dynamiques que Patroni.
*   **AWS RDS / Cloud Managé** : Rejeté pour des raisons de souveraineté et de déploiement "on-premise" requis par certains sites.
