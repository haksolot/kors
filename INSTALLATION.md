# Guide d'Installation KORS

Ce guide vous accompagne dans le déploiement de votre propre instance KORS en environnement de développement.

## Pré-requis

*   **Docker** et **Docker Compose**.
*   **Go 1.25+** (pour le développement local).
*   **Make** (optionnel, pour les raccourcis).

## Démarrage Rapide

### 1. Cloner le dépôt
```bash
git clone https://github.com/haksolot/kors.git
cd kors
```

### 2. Configurer l'environnement
Copiez le fichier d'exemple pour initialiser vos variables d'environnement locales :
```bash
cp .env.example .env
```

### 3. Lancer l'infrastructure
Utilisez Docker Compose pour démarrer tous les services (Postgres, NATS, MinIO, Jaeger, Keycloak, KORS API) :
```bash
docker-compose up -d --build
```

### 4. Appliquer les migrations SQL
Initialisez le schéma de la base de données :
```bash
make migrate
# OU manuellement
cd kors-api/migrations
go run github.com/pressly/goose/v3/cmd/goose postgres "postgres://kors:kors_dev_secret@localhost:5432/kors?sslmode=disable" up
```

## Vérification

Une fois le déploiement terminé, vous pouvez accéder aux services suivants :

*   **GraphQL Playground** : [http://localhost:8080](http://localhost:8080)
*   **Traces (Jaeger)** : [http://localhost:16686](http://localhost:16686)
*   **Fichiers (MinIO)** : [http://localhost:9001](http://localhost:9001) (Login: `kors_admin` / Pass: `kors_dev_secret`)
*   **SSO (Keycloak)** : [http://localhost:8180](http://localhost:8180) (Admin: `admin` / Pass: `admin`)

## Développement

### Régénération du code GraphQL
Si vous modifiez le schéma dans `shared/schema/kors.graphql`, lancez :
```bash
make generate
```

### Exécution des tests
Pour valider la stabilité du système :
```bash
make test
```
