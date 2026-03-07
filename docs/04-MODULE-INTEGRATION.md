# KORS Volume 4 : Intégration de Modules et Fédération

Ce document explique comment brancher un nouveau service métier sur l'écosystème KORS.

## 1. Étape 1 : Provisionnement
Avant de commencer, demandez vos identifiants à KORS.
*   Action : Appelez la mutation `provisionModule(moduleName: "mon_module")`.
*   Résultat : KORS crée un **schéma PostgreSQL dédié** et un **utilisateur propriétaire**.
*   Sécurité : Votre utilisateur a un droit `SELECT` sur le schéma `kors` mais aucun droit d'écriture (Lecture seule).

## 2. Étape 2 : Fédération (Subgraph)
KORS utilise **Apollo Federation v2**. Pour exposer vos données métier via KORS :

### A. Étendre le type Resource
Dans votre schéma GraphQL local, déclarez que vous étendez la ressource KORS :
```graphql
type Resource @key(fields: "id") {
  id: UUID! @external
  mon_champ_metier: String
}
```

### B. Implémenter le résolveur d'entité
Votre code doit être capable de renvoyer vos données locales à partir de l'UUID KORS. Le Gateway Apollo fera le lien automatiquement.

## 3. Étape 3 : Écoute du Bus (NATS)
Pour garder votre base locale synchronisée (ex: mise à jour des statuts), abonnez-vous aux événements de KORS :
*   `kors.resource.created`
*   `kors.resource.state_changed`

**Utilisez les Queue Groups** : Donnez le même `Durable Name` à toutes vos instances pour que NATS répartisse les messages sans doublon.

## 4. Pattern de création symbiotique
1.  Le client appelle votre module.
2.  Votre module appelle `kors-api` pour créer la ressource technique.
3.  Votre module reçoit l'UUID et l'utilise pour enregistrer les données riches dans son schéma local.
4.  Tout échec à l'étape 2 doit annuler l'étape 3.
