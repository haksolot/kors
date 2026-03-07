# KORS Volume 6 : Contexte Technique pour Agents IA

Ce document contient les méta-instructions et les invariants techniques pour les agents IA chargés de développer des modules tiers pour KORS.

---

## 1. Structure du Code et Invariants
*   **Monorepo Go Workspace** : Utilisez `go.work` pour résoudre les dépendances entre les services.
*   **Layer Domain** : Les fichiers dans `internal/domain` doivent rester purs (pas d'import SQL ou NATS).
*   **Dependency Injection** : Tout l'assemblage se fait dans `main.go`. N'utilisez jamais de variables globales.

## 2. Guide de Débogage Rapide
En cas d'erreur `internal system error` sur GraphQL :
1.  **Nil Pointer** : Vérifiez l'ordre d'initialisation dans `main.go`. L'objet Resolver doit être complet.
2.  **Auth Failure** : Assurez-vous d'envoyer le header `Authorization: Bearer <token>`. Sinon, l'identité injectée sera `uuid.Nil` et le RBAC échouera.
3.  **SQL Not Null** : Vérifiez que les métadonnées passées ne sont pas `nil`. Initialisez toujours une map vide en cas de doute.

## 3. Protocoles de Communication
*   **GraphQL Mutation Result** : Ne cherchez pas les erreurs métier dans le tableau d'erreurs standard de GraphQL. Cherchez toujours le champ `error { code message }` du type de retour.
*   **NATS Idempotence** : `kors-events` utilise l'ID de l'événement comme `Nats-Msg-Id`. Un événement déjà reçu ne sera pas retraité.

## 4. Invariants de la Base de Données
*   **Clé Primaire** : L'ID généré par `createResource` est l'unique identifiant légitime. Les modules ne doivent pas générer leurs propres IDs.
*   **Metadata** : KORS valide les métadonnées contre le JSON Schema défini dans le `ResourceType`. Toute structure invalide sera rejetée.

## 5. Checklist de Validation
Avant de livrer un changement, l'agent doit :
1.  **Compiler** : `go build ./cmd/server`.
2.  **Tester** : Lancer le service sur Docker.
3.  **Vérifier les Traces** : Consulter Jaeger (`localhost:16686`) pour valider le flux de la requête.
