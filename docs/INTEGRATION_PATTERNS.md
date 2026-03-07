# Patterns d'Intégration Avancés

Ce document guide les développeurs sur la manière de structurer leurs interactions avec KORS.

---

## 1. Pattern : La Ressource Maîtresse

Votre module ne doit JAMAIS générer son propre identifiant pour une entité qui nécessite une traçabilité EN9100.

### Flux de création recommandé :
1.  **Client** appelle votre module (REST/gRPC/GraphQL).
2.  **Votre module** appelle `kors-api` mutation `createResource`.
3.  **KORS** valide et renvoie un UUID.
4.  **Votre module** enregistre cet UUID comme ID de sa propre ligne métier.

```go
// Exemple d'implémentation robuste
func CreatePart(ctx context.Context, input PartInput) (uuid.UUID, error) {
    // Appel à KORS
    korsID, err := kors.CreateResource(ctx, "aircraft_part", "RAW_MATERIAL", input.Meta)
    if err != nil {
        return uuid.Nil, fmt.Errorf("KORS rejection: %w", err)
    }

    // Persistance locale
    err = db.Exec("INSERT INTO mes.parts (id, batch_number) VALUES ($1, $2)", korsID, input.Batch)
    return korsID, err
}
```

---

## 2. Pattern : Observateur d'État (NATS)

Pour garder vos vues synchronisées, votre module doit écouter les événements de KORS.

### Sujets à surveiller :
*   `kors.resource.created` : Utile pour indexer une nouvelle ressource dans une base de recherche (Elasticsearch).
*   `kors.resource.state_changed` : Critique pour mettre à jour les statuts dans votre UI ou déclencher des jobs métier (ex: impression d'étiquette lors du passage en `PROCESSED`).

---

## 3. Pattern : Validation JSON Schema

KORS utilise le format **JSON Schema Draft 7**. 
Assurez-vous que vos métadonnées respectent strictement le schéma déclaré lors du `registerResourceType`. KORS rejettera toute création/transition si un champ obligatoire manque ou si le type est incorrect (ex: string à la place d'un int).
