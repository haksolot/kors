# KORS SDK for Go

Le SDK KORS pour Go fournit un client GraphQL entièrement typé et asynchrone, généré via `genqlient`.

## 1. Installation

```bash
go get github.com/kors-project/kors/sdk/go
```

## 2. Initialisation

```go
import "github.com/kors-project/kors/sdk/go"

func main() {
    client := sdk.NewClient("http://localhost:8080/query", "YOUR_JWT_TOKEN")
    
    // Accès direct au client typé
    ctx := context.Background()
    resp, err := sdk.GetResource(ctx, client.GQL(), resourceID)
}
```

## 3. Exemples

### Création d'une ressource
```go
input := sdk.CreateResourceInput{
    TypeName:     "tool",
    InitialState: "idle",
    Metadata:     map[string]interface{}{"serial": "SN-123"},
}

resp, err := sdk.CreateResource(ctx, client.GQL(), input)
if err != nil {
    log.Fatal(err)
}

if resp.CreateResource.Success {
    fmt.Printf("Created resource ID: %s\n", resp.CreateResource.Resource.Id)
}
```

---

## Maintenance du SDK
Pour mettre à jour le SDK après un changement de schéma GraphQL :
1.  Mettre à jour `shared/schema/kors.graphql`.
2.  Lancer `go run github.com/Khan/genqlient` dans ce répertoire.
