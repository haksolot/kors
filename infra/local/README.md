# KORS — Environnement local de developpement

Ce repertoire contient toute l'infrastructure locale necessaire au developpement de KORS.
Il fait tourner en Docker Compose l'integralite de la stack, a l'exception de K3s qui est
reserve aux tests de deploiement Edge.

---

## Prerequis

- Docker >= 24.0
- Docker Compose >= 2.22 (support du champ `include`)
- Make

---

## Demarrage rapide

```bash
cd infra/local
cp .env .env.local          # optionnel : surcharger des variables localement
make up                     # demarre toute la stack
make health                 # verifie que tout est operationnel (~30s apres up)
```

---

## URLs et acces locaux

| Service | URL | Identifiants |
|---|---|---|
| **Traefik dashboard** | http://localhost:8090/dashboard/ | — |
| **Keycloak admin** | http://localhost:8888 | `admin` / `admin_dev_secret` |
| **MinIO console** | http://localhost:9001 | `kors_minio` / `kors_minio_secret` |
| **NATS monitoring** | http://localhost:8222 | — |
| **PostgreSQL** | `localhost:5432` | `kors` / `kors_dev_secret` |

---

## Utilisateurs de test (realm `kors`)

Ces utilisateurs sont crees automatiquement dans Keycloak au premier demarrage.
Chaque utilisateur correspond a un persona KORS avec son role RBAC.

| Utilisateur | Mot de passe | Role | Persona |
|---|---|---|---|
| `admin.kors` | `Admin1234!` | `kors-admin` | Admin systeme |
| `thomas.rq` | `Thomas1234!` | `kors-responsable-qualite` | Thomas — Responsable Qualite |
| `karim.di` | `Karim1234!` | `kors-directeur-industriel` | Karim — Directeur Industriel |
| `isabelle.op` | `Isabelle1234!` | `kors-operateur` | Isabelle — Operatrice bord de ligne |
| `marc.dsi` | `Marc1234!` | `kors-admin` | Marc — DSI (acces admin pour tests infra) |

---

## Roles RBAC

| Role | Acces |
|---|---|
| `kors-admin` | Acces complet a tous les modules et a la configuration |
| `kors-responsable-qualite` | QMS complet, plans de controle, NC, dossiers de conformite EN9100 |
| `kors-directeur-industriel` | MES complet, dashboard TRS, pilotage des OF, analytics |
| `kors-operateur` | Interface guidage uniquement : OF assignes, saisie temps et controles |

---

## Commandes utiles

```bash
make help            # liste toutes les commandes disponibles
make up              # demarre toute la stack
make down            # arrete (conserve les donnees)
make restart         # redemarrage complet
make logs            # logs en temps reel de tous les services
make ps              # etat des conteneurs
make health          # verifie les healthchecks de chaque service

# Demarrage par sous-ensemble
make up-nats         # NATS uniquement
make up-db           # PostgreSQL uniquement
make up-iam          # Keycloak (+ PostgreSQL en dependance)

# Connexion directe
make psql            # shell psql dans la base kors
make nats-cli        # shell dans le conteneur NATS
make nats-stream-list # liste les streams JetStream configures

# Nettoyage
make clean           # supprime les conteneurs (conserve les volumes)
make nuke            # DESTRUCTIF : supprime tout, y compris les volumes
```

---

## Structure des fichiers

```
infra/local/
├── docker-compose.yml          # Orchestrateur : network + volumes + includes
├── docker-compose.nats.yml     # NATS + JetStream
├── docker-compose.db.yml       # PostgreSQL 16 + TimescaleDB
├── docker-compose.storage.yml  # MinIO + init bucket
├── docker-compose.iam.yml      # Keycloak + import realm
├── docker-compose.gateway.yml  # Traefik reverse proxy
├── Makefile                    # Commandes de developpement
├── .env                        # Variables d'environnement (ne pas committer en prod)
├── config/
│   ├── nats/
│   │   └── nats.conf           # Config NATS : JetStream, WebSocket, Leaf Node (commente)
│   ├── keycloak/
│   │   └── realm-kors.json     # Realm KORS : roles, client OIDC, utilisateurs de test
│   ├── postgres/
│   │   └── init.sql            # Init : extensions TimescaleDB, uuid-ossp, schemas
│   └── traefik/
│       └── traefik.yml         # Config statique Traefik
└── README.md                   # Ce fichier
```

---

## Connexion OIDC depuis un service Go

Les variables necessaires pour connecter un service Go a Keycloak sont disponibles dans `.env` :

```
KEYCLOAK_ISSUER=http://keycloak:8080/realms/kors
KEYCLOAK_JWKS_URL=http://keycloak:8080/realms/kors/protocol/openid-connect/certs
KEYCLOAK_CLIENT_ID=kors-bff
KEYCLOAK_CLIENT_SECRET=kors_bff_client_secret_dev
```

Pour obtenir un token de test en ligne de commande :

```bash
curl -s -X POST http://localhost:8888/realms/kors/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=kors-bff" \
  -d "client_secret=kors_bff_client_secret_dev" \
  -d "username=thomas.rq" \
  -d "password=Thomas1234!" \
  | jq .access_token
```

---

## Ajout de l'observabilite (Horizon 2)

La stack Prometheus + Grafana + Loki + Tempo + OTel Collector est prevue mais hors scope
du MVP. Quand on l'activera, elle sera ajoutee dans un `docker-compose.observability.yml`
separe et incluse dans le compose principal via un profil Docker.

---

## Notes sur le Leaf Node NATS

Le fichier `config/nats/nats.conf` contient la configuration Leaf Node commentee.
Pour connecter ce noeud local a un Hub Cloud NATS (simulation Edge/Cloud) :

1. Decommenter le bloc `leaf` dans `nats.conf`
2. Renseigner l'URL et les credentials du Hub
3. `make restart` ou `make up-nats`

La synchronisation est automatique et transparente pour les services.
