# KORS Volume 4 : Guide du Développeur de Module

Ce guide détaille les étapes pour créer et intégrer un nouveau service métier dans l'écosystème KORS.

## Étape 1 : Obtenir des accès (Provisionnement)

Demandez à l'administrateur KORS (ou via votre pipeline de CI/CD) de provisionner votre module.
```graphql
mutation { provisionModule(moduleName: "votre_nom") { username password schema } }
```
Vous recevez des identifiants PostgreSQL dédiés. Votre utilisateur a tous les droits sur son schéma mais est en **lecture seule** sur le schéma `kors`.

## Étape 2 : Configurer l'Authentification

1.  Créez un Client dans **Keycloak** (nommé comme votre module).
2.  Activez les **Service Accounts**.
3.  Récupérez le **Client Secret**.
4.  Votre module doit échanger ce secret contre un JWT avant chaque appel à KORS (voir Volume 7).

## Étape 3 : Gestion des données (Le Modèle Symbiotique)

KORS gère la traçabilité, vous gérez le métier.

### Architecture SQL :
*   Vos tables doivent être dans votre schéma dédié.
*   Utilisez l'**UUID KORS** comme clé primaire.
```sql
CREATE TABLE votre_schema.objets (
    id UUID PRIMARY KEY, -- Cet ID vient de KORS
    donnee_metier_1 TEXT,
    ...
);
```

## Étape 4 : Écoute en temps réel (NATS)

Abonnez-vous aux sujets NATS pour réagir aux changements globaux :
*   `kors.resource.created` : Pour initialiser vos données locales.
*   `kors.resource.state_changed` : Pour mettre à jour vos statuts métier.

**Exemple Go (Abonnement Durable) :**
```go
sub, _ := js.PullSubscribe("kors.resource.state_changed", "votre-durable-name")
```

## Étape 5 : Fédération GraphQL (Optionnel mais recommandé)

Si vous voulez que vos données métier apparaissent directement dans l'API globale :
1.  Exposez un serveur GraphQL Subgraph.
2.  "Étendez" le type `Resource` de KORS.
```graphql
type Resource @key(fields: "id") {
  id: UUID! @external
  votreChampMetier: String
}
```
Le Gateway Apollo fusionnera automatiquement vos données avec celles du noyau.
