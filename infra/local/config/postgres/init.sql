-- =============================================================================
-- KORS — Initialisation PostgreSQL
-- Execute une seule fois au premier demarrage du conteneur.
-- Active l'extension TimescaleDB sur la base kors.
-- =============================================================================

-- Activation de TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- PG18 : uuidv7() est natif, uuid-ossp n'est plus necessaire.
-- Utiliser uuidv7() comme valeur par defaut des PKs dans les migrations goose.
-- Exemple : id UUID PRIMARY KEY DEFAULT uuidv7()

-- Extension pg_stat_statements pour le monitoring des requetes lentes
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Schema metier principal
CREATE SCHEMA IF NOT EXISTS mes;
CREATE SCHEMA IF NOT EXISTS qms;

-- Commentaire de documentation
COMMENT ON DATABASE kors IS 'Base de donnees principale KORS — MES + QMS + Outbox';
