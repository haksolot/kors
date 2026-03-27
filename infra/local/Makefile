# =============================================================================
# KORS — Makefile infra locale
# =============================================================================

COMPOSE = docker compose --env-file .env
COMPOSE_FILES = -f docker-compose.yml

.PHONY: help up down restart logs ps clean nuke \
        up-nats up-db up-storage up-iam up-gateway \
        psql nats-cli minio-cli

# --- Aide ---

help: ## Affiche cette aide
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  URLs locales :"
	@echo "    Traefik dashboard  http://localhost:8090/dashboard/"
	@echo "    Keycloak admin     http://localhost:8888  (admin / admin_dev_secret)"
	@echo "    MinIO console      http://localhost:9001  (kors_minio / kors_minio_secret)"
	@echo "    NATS monitor       http://localhost:8222"
	@echo "    PostgreSQL         localhost:5432  (kors / kors_dev_secret)"

# --- Stack complete ---

up: ## Demarre toute la stack
	$(COMPOSE) $(COMPOSE_FILES) up -d
	@echo ""
	@echo "Stack KORS demarree. Attendre ~30s pour que Keycloak soit pret."
	@echo "Verifier l'etat : make ps"

down: ## Arrete toute la stack (conserve les volumes)
	$(COMPOSE) $(COMPOSE_FILES) down

restart: ## Redemarrage complet
	$(MAKE) down
	$(MAKE) up

logs: ## Affiche les logs de tous les services (Ctrl+C pour quitter)
	$(COMPOSE) $(COMPOSE_FILES) logs -f

ps: ## Etat de tous les conteneurs
	$(COMPOSE) $(COMPOSE_FILES) ps

# --- Demarrage par service ---

up-nats: ## Demarre uniquement NATS
	$(COMPOSE) $(COMPOSE_FILES) up -d nats

up-db: ## Demarre uniquement PostgreSQL + TimescaleDB
	$(COMPOSE) $(COMPOSE_FILES) up -d postgres

up-storage: ## Demarre MinIO + init bucket
	$(COMPOSE) $(COMPOSE_FILES) up -d minio minio-init

up-iam: ## Demarre Keycloak (necessite postgres)
	$(COMPOSE) $(COMPOSE_FILES) up -d postgres keycloak

up-gateway: ## Demarre Traefik
	$(COMPOSE) $(COMPOSE_FILES) up -d traefik

# --- Logs par service ---

logs-nats: ## Logs NATS
	$(COMPOSE) $(COMPOSE_FILES) logs -f nats

logs-db: ## Logs PostgreSQL
	$(COMPOSE) $(COMPOSE_FILES) logs -f postgres

logs-keycloak: ## Logs Keycloak
	$(COMPOSE) $(COMPOSE_FILES) logs -f keycloak

logs-minio: ## Logs MinIO
	$(COMPOSE) $(COMPOSE_FILES) logs -f minio

logs-traefik: ## Logs Traefik
	$(COMPOSE) $(COMPOSE_FILES) logs -f traefik

# --- Outils de connexion ---

psql: ## Ouvre un shell psql dans la base kors
	docker exec -it kors-postgres psql -U kors -d kors

nats-cli: ## Ouvre un shell dans le conteneur NATS (nats CLI disponible)
	docker exec -it kors-nats sh

nats-stream-list: ## Liste les streams JetStream configures
	docker exec kors-nats nats stream list --server nats://localhost:4222 2>/dev/null || \
		echo "Aucun stream configure (normal au premier demarrage)"

minio-cli: ## Ouvre un shell mc (MinIO client) configure sur l'instance locale
	docker run --rm -it --network kors-net \
		--entrypoint sh \
		-e "MC_HOST_local=http://kors_minio:kors_minio_secret@minio:9000" \
		minio/mc:latest

# --- Reinitialisation ---

clean: ## Arrete et supprime les conteneurs (conserve les volumes)
	$(COMPOSE) $(COMPOSE_FILES) down --remove-orphans

nuke: ## DESTRUCTIF — supprime conteneurs ET volumes (repart de zero)
	@echo "ATTENTION : toutes les donnees locales seront perdues."
	@read -p "Confirmer ? (oui) : " confirm && [ "$$confirm" = "oui" ] || exit 1
	$(COMPOSE) $(COMPOSE_FILES) down --volumes --remove-orphans
	@echo "Stack et volumes supprimes."

# --- Healthchecks ---

health: ## Verifie l'etat de sante de chaque service
	@echo "--- NATS ---"
	@curl -sf http://localhost:8222/healthz && echo " OK" || echo " KO"
	@echo "--- PostgreSQL ---"
	@docker exec kors-postgres pg_isready -U kors -d kors && echo " OK" || echo " KO"
	@echo "--- Keycloak ---"
	@curl -sf http://localhost:8888/health/ready && echo " OK" || echo " KO"
	@echo "--- MinIO ---"
	@curl -sf http://localhost:9000/minio/health/live && echo " OK" || echo " KO"
	@echo "--- Traefik ---"
	@curl -sf http://localhost:8090/ping && echo " OK" || echo " KO"
