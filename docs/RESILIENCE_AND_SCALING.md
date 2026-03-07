# Stratégie de Résilience et de Mise à l'Échelle KORS

Ce document définit l'architecture cible pour le déploiement industriel de KORS, garantissant la haute disponibilité (HA), la tolérance aux pannes et l'absence de conflits de données dans un environnement multi-instances.

---

## 1. Topologie Globale du Système

KORS est conçu pour fonctionner sous forme de **réseau uniforme**. Chaque composant doit être capable de s'exécuter en plusieurs exemplaires simultanés sans dégrader l'intégrité du système.

### Schéma de principe
`Trafic Client` → **[Load Balancer / Ingress]** → `N x kors-api` → **[Postgres Cluster / NATS Cluster / MinIO Cluster]**

---

## 2. Composants Applicatifs (Stateless)

### KORS-API
*   **Mode** : Actif / Actif.
*   **Scaling** : Horizontal (on rajoute des pods selon la charge).
*   **Load Balancing** : Round-Robin au niveau du Load Balancer.
*   **Gestion des Conflits** : Aucune donnée n'est stockée localement. L'API délègue la cohérence à PostgreSQL via des transactions atomiques.

### KORS-EVENTS
*   **Mode** : Actif / Actif avec **Queue Groups**.
*   **Mécanisme** : Utilisation des `QueueGroups` NATS. Si 3 instances de `kors-events` sont lancées, NATS garantit qu'un événement donné n'est délivré qu'à **une seule** des trois instances.
*   **Avantage** : Répartition de la charge de traitement et pas de double notification.

### KORS-WORKER
*   **Mode** : Actif / Passif ou Actif / Actif avec Verrous.
*   **Mécanisme** : Pour éviter que deux workers nettoient la même permission, nous utilisons la clause SQL `SELECT ... FOR UPDATE SKIP LOCKED`. 
*   **Résultat** : Les workers se partagent les tâches sans jamais se marcher dessus.

---

## 3. Bus d'Événements (NATS JetStream)

La résilience du bus repose sur l'algorithme de consensus **Raft**.

*   **Cluster** : Minimum 3 nœuds NATS.
*   **Réplication** : Chaque message publié sur le stream `KORS` est répliqué sur au moins 2 nœuds (Quorum).
*   **Garantie** : Un message n'est acquitté par le bus que s'il est sécurisé sur disque par la majorité des nœuds. Si un serveur NATS tombe, le flux continue sans perte.

---

## 4. Source de Vérité (PostgreSQL Cluster)

La base de données est le seul point "stateful" nécessitant une gestion fine du Leader.

*   **Technologie** : **Patroni** + **etcd**.
*   **Architecture** : 1 Leader (Écriture/Lecture) + 2 Replicas (Lecture seule).
*   **Failover** : En cas de chute du Leader, Patroni élit un nouveau Leader en < 10 secondes.
*   **Connexion** : Les applications passent par **PgBouncer** pour une gestion efficace des connexions et un basculement transparent vers le nouveau Leader.

---

## 5. Stockage Objet (MinIO Distributed)

Le stockage des fichiers (plans, révisions) utilise le mode **Distributed MinIO**.

*   **Mécanisme** : **Erasure Coding**.
*   **Principe** : Les fichiers sont découpés en blocs de données et blocs de parité répartis sur N disques/serveurs.
*   **Résilience** : Le cluster peut perdre jusqu'à la moitié des disques sans aucune perte de données et sans interruption de service.

---

## 6. Table de Synthèse de la Résilience

| Composant | Stratégie | Outil | Solution aux Conflits |
|---|---|---|---|
| **API** | Stateless Multi-instance | Kubernetes Service | Transactions SQL |
| **Bus** | Consensus Distributed | NATS JetStream (Raft) | Sequence Numbers |
| **Base de Données** | HA Leader/Replica | Patroni / etcd | Transactions ACID |
| **Traitement** | Consumer Groups | NATS Queue Groups | Distribution 1-par-1 |
| **Stockage** | Distributed File System | MinIO Erasure Coding | ID de fichier immuable |

---

## 7. Plan de Mise en Œuvre (Phase 2 & 3)

1.  **Déploiement K8s** : Utilisation de Helm Charts pour déployer les clusters NATS et Postgres.
2.  **Instrumentation** : Monitoring du décalage de réplication (Replication Lag) et du quorum NATS via Prometheus/Grafana.
3.  **Tests de Chaos** : Simulation de pannes (arrêt brutal d'un nœud Postgres ou NATS) pour valider le basculement automatique sans perte de données.
