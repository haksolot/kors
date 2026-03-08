# GEMINI.md — Directives d'amelioration de KORS

Ce fichier est ta feuille de route. Tu as acces a l'integralite de la codebase KORS.
Suis les taches dans l'ordre de priorite indique. Chaque tache inclut la localisation exacte
des fichiers a modifier, le comportement actuel, le comportement attendu, et le code de reference
ou les patterns a appliquer.

---

## Contexte du projet

KORS est un noyau technique Go (1.25) structure en architecture hexagonale :
- `kors-api/` — API GraphQL principale (gqlgen, Chi, pgx)
- `kors-events/` — Consommateur NATS JetStream
- `kors-worker/` — Jobs de maintenance (purge RBAC)
- `sdk/` — Clients Go, TypeScript, Python
- `shared/` — Packages partages (korsctx, pagination, tracing)
- `examples/module-example/` — Module TMS de reference

Module Go racine : `github.com/haksolot/kors/kors-api`
Workspace Go : `go.work` a la racine du repo.

---

## PRIORITE 1 — Securite (Production blockers)

### TACHE 1.1 — Validation JWT cryptographique dans le middleware Auth

**Fichier** : `kors-api/internal/middleware/auth.go`

**Probleme actuel** :
La methode `new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})` ne valide PAS
la signature cryptographique du token. N'importe quel token forge est accepte.

**Ce que tu dois faire** :

1. Ajouter une variable d'environnement `JWKS_ENDPOINT` dans la configuration (lue dans `cmd/server/main.go`).

2. Creer un fichier `kors-api/internal/middleware/jwks.go` qui implemente un fetcher de cles
   publiques avec cache (TTL 1h) :

```go
package middleware

import (
    "context"
    "crypto/rsa"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "math/big"
    "net/http"
    "sync"
    "time"
)

type JWKSCache struct {
    mu        sync.RWMutex
    keys      map[string]*rsa.PublicKey
    fetchedAt time.Time
    ttl       time.Duration
    endpoint  string
}

func NewJWKSCache(endpoint string, ttl time.Duration) *JWKSCache {
    return &JWKSCache{endpoint: endpoint, ttl: ttl, keys: make(map[string]*rsa.PublicKey)}
}

// GetKey retourne la cle publique RSA pour un kid donne.
// Rafraichit le cache si expire.
func (c *JWKSCache) GetKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
    c.mu.RLock()
    if time.Since(c.fetchedAt) < c.ttl {
        if key, ok := c.keys[kid]; ok {
            c.mu.RUnlock()
            return key, nil
        }
    }
    c.mu.RUnlock()
    return c.refresh(ctx, kid)
}

func (c *JWKSCache) refresh(ctx context.Context, kid string) (*rsa.PublicKey, error) {
    c.mu.Lock()
    defer c.mu.Unlock()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
    if err != nil {
        return nil, err
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
    }
    defer resp.Body.Close()

    var jwks struct {
        Keys []struct {
            Kid string `json:"kid"`
            N   string `json:"n"`
            E   string `json:"e"`
        } `json:"keys"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
        return nil, fmt.Errorf("failed to decode JWKS: %w", err)
    }

    c.keys = make(map[string]*rsa.PublicKey)
    c.fetchedAt = time.Now()

    for _, k := range jwks.Keys {
        nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
        if err != nil {
            continue
        }
        eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
        if err != nil {
            continue
        }
        e := int(new(big.Int).SetBytes(eBytes).Int64())
        pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}
        c.keys[k.Kid] = pub
    }

    if key, ok := c.keys[kid]; ok {
        return key, nil
    }
    return nil, fmt.Errorf("key %q not found in JWKS", kid)
}
```

3. Modifier `AuthMiddleware` pour recevoir `*JWKSCache` et valider les tokens JWT de maniere
   cryptographique :

```go
// Structure mise a jour
type AuthMiddleware struct {
    IdentityRepo identity.Repository
    JWKSCache    *JWKSCache // Nouveau champ
}

// Dans Handler(), remplacer le bloc "Real JWT Parsing" par :
token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
    }
    kid, _ := token.Header["kid"].(string)
    return m.JWKSCache.GetKey(r.Context(), kid)
})
if err != nil || !token.Valid {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
claims, ok := token.Claims.(jwt.MapClaims)
if !ok {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
// Verifier l'expiration (jwt.Parse le fait deja, mais expliciter)
externalID, _ := claims["sub"].(string)
// ... suite identique
```

4. Mettre a jour le wiring dans `kors-api/cmd/server/main.go` :

```go
jwksEndpoint := getEnv("JWKS_ENDPOINT", "http://kors-sso:8080/realms/kors/protocol/openid-connect/certs")
jwksCache := korsauth.NewJWKSCache(jwksEndpoint, time.Hour)
// Passer jwksCache a AuthMiddleware
authMiddleware := &korsauth.AuthMiddleware{IdentityRepo: idRepo, JWKSCache: jwksCache}
```

5. En mode developpement (`APP_ENV=development`), conserver le bypass `mock-` et `system`
   UNIQUEMENT si `JWKS_ENDPOINT` n'est pas defini. En production, ces bypasses doivent
   retourner 401.

**Variables d'environnement a documenter dans `.env.example`** :
```
JWKS_ENDPOINT=http://kors-sso:8080/realms/kors/protocol/openid-connect/certs
APP_ENV=development
```

---

### TACHE 1.2 — Generation de mot de passe cryptographiquement sure

**Fichier** : `kors-api/internal/adapter/postgres/provisioner.go`

**Probleme actuel** :
```go
// MAUVAIS - math/rand est predictible
rand.Seed(time.Now().UnixNano())
b[i] = letters[rand.Intn(len(letters))]
```

**Remplacement exact de la fonction `generateRandomPassword`** :
```go
import "crypto/rand"

func generateRandomPassword(n int) string {
    const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        panic(fmt.Sprintf("crypto/rand failed: %v", err))
    }
    for i := range b {
        b[i] = letters[int(b[i])%len(letters)]
    }
    return string(b)
}
```

Supprimer l'import `"math/rand"` et `"time"` s'ils ne sont plus utilises dans ce fichier.

---

### TACHE 1.3 — SQL Injection dans le provisionnement

**Fichier** : `kors-api/internal/adapter/postgres/provisioner.go`

**Probleme actuel** :
`moduleName` est insere directement dans des requetes DDL via `fmt.Sprintf`. Un nom de module
malveillant comme `"; DROP SCHEMA kors CASCADE; --` serait execute.

**Ce que tu dois faire** :

1. Ajouter une fonction de validation du nom de module au debut du fichier :

```go
import "regexp"

var moduleNameRegex = regexp.MustCompile(`^[a-z][a-z0-9_]{1,30}$`)

func validateModuleName(name string) error {
    if !moduleNameRegex.MatchString(name) {
        return fmt.Errorf(
            "invalid module name %q: must match ^[a-z][a-z0-9_]{1,30}$ (lowercase, start with letter, max 30 chars)",
            name,
        )
    }
    return nil
}
```

2. Appeler cette validation en debut de `ProvisionModule` et `DeprovisionModule` :

```go
func (p *PostgresProvisioner) ProvisionModule(ctx context.Context, moduleName string) (*provisioning.ModuleCredentials, error) {
    if err := validateModuleName(moduleName); err != nil {
        return nil, err
    }
    // ... suite
}

func (p *PostgresProvisioner) DeprovisionModule(ctx context.Context, moduleName string) error {
    if err := validateModuleName(moduleName); err != nil {
        return err
    }
    // ... suite
}
```

3. Appliquer la meme validation dans le UseCase `provision_module.go` avant d'appeler le Provisioner.

---

## PRIORITE 2 — Corrections de bugs critiques

### TACHE 2.1 — Transactions SQL atomiques dans CreateResource

**Fichier** : `kors-api/internal/usecase/create_resource.go`

**Probleme actuel** :
La resource est persistee en DB, puis l'evenement est cree, puis NATS est appele. Si NATS
echoue, la resource existe en DB mais aucun evenement n'a ete emis. La DB est inconsistante.
Le commentaire dans le code dit lui-meme "TODO: implement real transaction".

**Ce que tu dois faire** :

1. Modifier l'interface du `CreateResourceUseCase` pour injecter un `*pgxpool.Pool` au lieu de
   l'interface `DB` actuelle qui ne fonctionne pas :

```go
import "github.com/jackc/pgx/v5/pgxpool"

type CreateResourceUseCase struct {
    Pool             *pgxpool.Pool
    ResourceRepo     resource.Repository
    ResourceTypeRepo resourcetype.Repository
    EventRepo        event.Repository
    PermissionRepo   permission.Repository
    EventPublisher   event.Publisher
}
```

2. Reecrire la methode `Execute` avec une vraie transaction pgx :

```go
func (uc *CreateResourceUseCase) Execute(ctx context.Context, input CreateResourceInput) (*resource.Resource, error) {
    rt, err := uc.ResourceTypeRepo.GetByName(ctx, input.TypeName)
    if err != nil {
        return nil, fmt.Errorf("failed to lookup resource type: %w", err)
    }
    if rt == nil {
        return nil, fmt.Errorf("resource type %q not found", input.TypeName)
    }

    allowed, err := uc.PermissionRepo.Check(ctx, input.IdentityID, "write", nil, &rt.ID)
    if err != nil {
        return nil, fmt.Errorf("failed to check permission: %w", err)
    }
    if !allowed {
        return nil, fmt.Errorf("identity %s does not have 'write' permission on type %s", input.IdentityID, rt.Name)
    }

    // Valider le JSON Schema des metadata
    if err := rt.ValidateMetadata(input.Metadata); err != nil {
        return nil, fmt.Errorf("metadata validation failed: %w", err)
    }

    if input.Metadata == nil {
        input.Metadata = make(map[string]interface{})
    }

    res := &resource.Resource{
        ID:        uuid.New(),
        TypeID:    rt.ID,
        State:     input.InitialState,
        Metadata:  input.Metadata,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }

    ev := &event.Event{
        ID:         uuid.New(),
        ResourceID: &res.ID,
        IdentityID: input.IdentityID,
        Type:       "kors.resource.created",
        Payload:    map[string]interface{}{"type": rt.Name, "state": res.State},
        CreatedAt:  time.Now(),
    }

    // Transaction SQL : resource + event en atomique
    tx, err := uc.Pool.Begin(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback(ctx) // No-op si Commit reussit

    pgRepo, ok := uc.ResourceRepo.(*postgresAdapter.ResourceRepository)
    if !ok {
        return nil, fmt.Errorf("internal error: ResourceRepo must be *postgres.ResourceRepository for transactional writes")
    }
    if err := pgRepo.CreateWithTx(ctx, tx, res); err != nil {
        return nil, fmt.Errorf("failed to persist resource: %w", err)
    }

    pgEventRepo, ok := uc.EventRepo.(*postgresAdapter.EventRepository)
    if !ok {
        return nil, fmt.Errorf("internal error: EventRepo must be *postgres.EventRepository for transactional writes")
    }
    if err := pgEventRepo.CreateWithTx(ctx, tx, ev); err != nil {
        return nil, fmt.Errorf("failed to persist event: %w", err)
    }

    // Publier sur NATS AVANT le commit : si NATS echoue, on rollback
    if uc.EventPublisher != nil {
        if err := uc.EventPublisher.Publish(ctx, ev); err != nil {
            return nil, fmt.Errorf("failed to publish event to bus (transaction aborted): %w", err)
        }
    }

    // Tout s'est bien passe, on commit
    if err := tx.Commit(ctx); err != nil {
        return nil, fmt.Errorf("failed to commit transaction: %w", err)
    }

    return res, nil
}
```

3. Ajouter `CreateWithTx(ctx context.Context, tx pgx.Tx, e *Event) error` sur `EventRepository`
   dans `kors-api/internal/adapter/postgres/event_repo.go` en suivant le meme pattern que
   `ResourceRepository.CreateWithTx` qui existe deja.

4. Mettre a jour le wiring dans `resolvers/resolver.go` pour passer `pool` au lieu de l'interface `DB`.

---

### TACHE 2.2 — Validation du JSON Schema des metadata

**Fichiers** :
- `kors-api/internal/domain/resourcetype/resourcetype.go`
- `kors-api/internal/usecase/create_resource.go`
- `kors-api/internal/usecase/transition_resource.go`

**Probleme actuel** :
Le champ `json_schema` est stocke dans `ResourceType` mais n'est jamais utilise pour valider
les `metadata` lors de `createResource` ou `transitionResource`.

**Ce que tu dois faire** :

1. Ajouter la dependance dans `kors-api/go.mod` :
```
github.com/xeipuuv/gojsonschema v1.2.0
```

2. Ajouter la methode `ValidateMetadata` sur `ResourceType` dans
   `kors-api/internal/domain/resourcetype/resourcetype.go` :

```go
import "github.com/xeipuuv/gojsonschema"

// ValidateMetadata valide les metadata contre le json_schema du type.
// Retourne nil si le schema est vide (pas de contrainte) ou si la validation passe.
func (rt *ResourceType) ValidateMetadata(metadata map[string]interface{}) error {
    if len(rt.JSONSchema) == 0 {
        return nil
    }
    schemaLoader := gojsonschema.NewGoLoader(rt.JSONSchema)
    documentLoader := gojsonschema.NewGoLoader(metadata)
    result, err := gojsonschema.Validate(schemaLoader, documentLoader)
    if err != nil {
        return fmt.Errorf("schema validation error: %w", err)
    }
    if !result.Valid() {
        errs := make([]string, len(result.Errors()))
        for i, e := range result.Errors() {
            errs[i] = e.String()
        }
        return fmt.Errorf("metadata does not match schema: %s", strings.Join(errs, "; "))
    }
    return nil
}
```

3. Appeler `rt.ValidateMetadata(input.Metadata)` dans :
   - `CreateResourceUseCase.Execute` (apres la recuperation du ResourceType)
   - `TransitionResourceUseCase.Execute` (apres la recuperation du ResourceType, avant `Update`)

---

### TACHE 2.3 — Resolvers Query manquants

**Fichier** : `kors-api/internal/graph/resolvers/schema.resolvers.go`

**Probleme actuel** :
Plusieurs resolvers Query retournent `nil, nil` :

```go
func (r *queryResolver) Resource(ctx context.Context, id uuid.UUID) (*model.Resource, error) {
    return nil, nil // VIDE
}
func (r *queryResolver) ResourceType(ctx context.Context, name string) (*model.ResourceType, error) {
    return nil, nil // VIDE
}
func (r *queryResolver) ResourceTypes(ctx context.Context) ([]*model.ResourceType, error) {
    return nil, nil // VIDE
}
```

**Ce que tu dois faire** :

1. Ajouter `GetResourceUseCase` et `GetResourceTypeUseCase` dans la struct `Resolver`
   (`resolvers/resolver.go`).

2. Creer `kors-api/internal/usecase/get_resource.go` :

```go
package usecase

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
    "github.com/haksolot/kors/kors-api/internal/domain/resource"
)

type GetResourceUseCase struct {
    ResourceRepo   resource.Repository
    PermissionRepo permission.Repository
}

func (uc *GetResourceUseCase) Execute(ctx context.Context, id uuid.UUID, identityID uuid.UUID) (*resource.Resource, error) {
    res, err := uc.ResourceRepo.GetByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get resource: %w", err)
    }
    if res == nil {
        return nil, nil
    }
    allowed, err := uc.PermissionRepo.Check(ctx, identityID, "read", &res.ID, &res.TypeID)
    if err != nil {
        return nil, fmt.Errorf("failed to check permission: %w", err)
    }
    if !allowed {
        return nil, fmt.Errorf("permission denied")
    }
    return res, nil
}
```

3. Creer `kors-api/internal/usecase/get_resource_type.go` :

```go
package usecase

import (
    "context"
    "fmt"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
)

type GetResourceTypeUseCase struct {
    Repo resourcetype.Repository
}

func (uc *GetResourceTypeUseCase) ExecuteByName(ctx context.Context, name string) (*resourcetype.ResourceType, error) {
    rt, err := uc.Repo.GetByName(ctx, name)
    if err != nil {
        return nil, fmt.Errorf("failed to get resource type: %w", err)
    }
    return rt, nil
}

func (uc *GetResourceTypeUseCase) ExecuteList(ctx context.Context) ([]*resourcetype.ResourceType, error) {
    rts, err := uc.Repo.List(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to list resource types: %w", err)
    }
    return rts, nil
}
```

4. Implementer les resolvers dans `schema.resolvers.go` en mappant vers les nouveaux use cases.
   Mapper les structs domain vers les structs `model.*` generees par gqlgen.

5. Implémenter le resolver `Resource` dans `entity.resolvers.go` pour la resolution fedéree
   (la méthode `FindResourceByID` est probablement déjà générée dans ce fichier).

---

### TACHE 2.4 — Mutation createIdentity manquante dans le schema GraphQL

**Fichiers** :
- `kors-api/internal/graph/schema/schema.graphql`
- `kors-api/internal/graph/resolvers/schema.resolvers.go`
- `kors-api/internal/usecase/` (nouveau fichier)

**Probleme actuel** :
Le Volume 7 de la doc et le guide module decrivent une mutation `createIdentity` necessaire
pour l'onboarding des modules, mais elle n'existe pas dans le schema GraphQL.

**Ce que tu dois faire** :

1. Ajouter dans `schema.graphql` :

```graphql
input CreateIdentityInput {
  externalId: String!
  name: String!
  type: String! # 'user', 'service', 'system'
  metadata: JSON
}

type IdentityResult {
  success: Boolean!
  identity: Identity
  error: MutationError
}

# Dans type Mutation, ajouter :
"""
Registers a new identity (user, service, or system) in the KORS registry.
Admin permission required.
"""
createIdentity(input: CreateIdentityInput!): IdentityResult!
```

2. Creer `kors-api/internal/usecase/create_identity.go` :

```go
package usecase

import (
    "context"
    "fmt"
    "time"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/identity"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
)

var validIdentityTypes = map[string]bool{"user": true, "service": true, "system": true}

type CreateIdentityInput struct {
    ExternalID string
    Name       string
    Type       string
    Metadata   map[string]interface{}
    CallerID   uuid.UUID // L'identite qui fait l'appel
}

type CreateIdentityUseCase struct {
    Repo           identity.Repository
    PermissionRepo permission.Repository
}

func (uc *CreateIdentityUseCase) Execute(ctx context.Context, input CreateIdentityInput) (*identity.Identity, error) {
    // Seul un admin peut creer des identites
    allowed, err := uc.PermissionRepo.Check(ctx, input.CallerID, "admin", nil, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to check permission: %w", err)
    }
    if !allowed {
        return nil, fmt.Errorf("admin permission required to create identities")
    }

    if !validIdentityTypes[input.Type] {
        return nil, fmt.Errorf("invalid identity type %q: must be one of user, service, system", input.Type)
    }
    if input.ExternalID == "" || input.Name == "" {
        return nil, fmt.Errorf("externalId and name are required")
    }

    // Verifier l'unicite
    existing, err := uc.Repo.GetByExternalID(ctx, input.ExternalID)
    if err != nil {
        return nil, fmt.Errorf("failed to check existing identity: %w", err)
    }
    if existing != nil {
        return nil, fmt.Errorf("identity with externalId %q already exists", input.ExternalID)
    }

    id := &identity.Identity{
        ID:         uuid.New(),
        ExternalID: input.ExternalID,
        Name:       input.Name,
        Type:       input.Type,
        Metadata:   input.Metadata,
        CreatedAt:  time.Now(),
        UpdatedAt:  time.Now(),
    }

    if err := uc.Repo.Create(ctx, id); err != nil {
        return nil, fmt.Errorf("failed to create identity: %w", err)
    }
    return id, nil
}
```

3. Regenerer gqlgen apres modification du schema : `go run github.com/99designs/gqlgen generate`
   depuis `kors-api/`.

4. Implementer le resolver `CreateIdentity` dans `schema.resolvers.go`.

---

### TACHE 2.5 — Soft-delete effectif sur les resources

**Fichiers** :
- `kors-api/internal/domain/resource/resource.go`
- `kors-api/internal/adapter/postgres/resource_repo.go`
- `kors-api/internal/graph/schema/schema.graphql`
- `kors-api/internal/usecase/` (nouveau fichier)

**Probleme actuel** :
Le champ `deleted_at` existe en DB et dans la struct `Resource`, mais aucune mutation ne permet
de supprimer une resource, et la query `List` ne filtre pas les resources supprimees.

**Ce que tu dois faire** :

1. Modifier `resource_repo.go` : la query `List` et `GetByID` doivent filtrer `WHERE deleted_at IS NULL`.

2. Ajouter dans `schema.graphql` :

```graphql
# Dans type Mutation :
"""
Soft-deletes a resource. The resource is hidden from all queries but preserved for audit trail.
Requires 'admin' permission on the resource or its type.
"""
deleteResource(id: UUID!): ResourceResult!
```

3. Creer `kors-api/internal/usecase/delete_resource.go` avec verification de permission
   `admin` sur la resource ou son type.

4. Ajouter `SoftDelete(ctx context.Context, id uuid.UUID) error` sur `resource.Repository`
   et son implementation dans `resource_repo.go` :

```go
func (r *ResourceRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
    _, err := r.Pool.Exec(ctx,
        "UPDATE kors.resources SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL",
        id,
    )
    return err
}
```

---

### TACHE 2.6 — Pagination cursor-based correcte dans ListResources

**Fichier** : `kors-api/internal/usecase/list_resources.go`

**Probleme actuel** :
```go
// Le commentaire dit "assume the cursor is just the raw UUID for simplicity"
id, err := uuid.Parse(*input.After)
```
Le package `shared/pagination` existe avec `EncodeCursor` / `DecodeCursor` mais n'est pas utilise.

**Ce que tu dois faire** :

1. Modifier `list_resources.go` pour utiliser `pagination.DecodeCursor` :

```go
import "github.com/haksolot/kors/shared/pagination"

if input.After != nil {
    rawID, err := pagination.DecodeCursor(*input.After)
    if err != nil {
        return nil, fmt.Errorf("invalid cursor: %w", err)
    }
    id, err := uuid.Parse(rawID)
    if err != nil {
        return nil, fmt.Errorf("invalid cursor ID: %w", err)
    }
    afterID = &id
}
```

2. Modifier le resolver `Resources` dans `schema.resolvers.go` pour encoder les cursors :

```go
import "github.com/haksolot/kors/shared/pagination"

edges[i] = &model.ResourceEdge{
    Cursor: pagination.EncodeCursor(res.ID.String()),
    Node:   &model.Resource{...},
}
```

---

## PRIORITE 3 — Tests unitaires et d'integration

### TACHE 3.1 — Infrastructure de tests partagee (testhelper)

**Creer** : `kors-api/internal/testhelper/db.go`

Ce package sera utilise par tous les tests d'integration. Il demarre un PostgreSQL via
testcontainers et applique les migrations :

```go
package testhelper

import (
    "context"
    "database/sql"
    "path/filepath"
    "runtime"
    "testing"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    _ "github.com/jackc/pgx/v5/stdlib"
    "github.com/pressly/goose/v3"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)

// SetupTestDB demarre un conteneur PostgreSQL, applique les migrations,
// et retourne un pool pgx pret a l'emploi.
// Le conteneur est automatiquement termine en fin de test via t.Cleanup.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
    t.Helper()
    ctx := context.Background()

    pgContainer, err := postgres.Run(ctx,
        "postgres:15-alpine",
        postgres.WithDatabase("kors_test"),
        postgres.WithUsername("kors"),
        postgres.WithPassword("password"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second),
        ),
    )
    require.NoError(t, err)

    t.Cleanup(func() {
        _ = pgContainer.Terminate(ctx)
    })

    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    // Appliquer les migrations
    db, err := sql.Open("pgx", connStr)
    require.NoError(t, err)
    defer db.Close()

    // Chemin absolu vers les migrations depuis ce fichier
    _, filename, _, _ := runtime.Caller(0)
    migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
    require.NoError(t, goose.Up(db, migrationsDir))

    pool, err := pgxpool.New(ctx, connStr)
    require.NoError(t, err)

    t.Cleanup(pool.Close)

    return pool
}
```

---

### TACHE 3.2 — Tests des domaines (unit tests purs, sans DB)

**Creer** : `kors-api/internal/domain/resourcetype/resourcetype_test.go`

```go
package resourcetype_test

import (
    "testing"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
    "github.com/stretchr/testify/assert"
)

func TestCanTransitionTo(t *testing.T) {
    rt := &resourcetype.ResourceType{
        Transitions: map[string]interface{}{
            "idle":        []interface{}{"in_use", "maintenance"},
            "in_use":      []interface{}{"idle", "error"},
            "maintenance": []interface{}{"idle"},
            "error":       []interface{}{"idle", "maintenance"},
        },
    }

    tests := []struct {
        name      string
        from, to  string
        expected  bool
    }{
        {"allowed: idle -> in_use", "idle", "in_use", true},
        {"allowed: idle -> maintenance", "idle", "maintenance", true},
        {"allowed: in_use -> error", "in_use", "error", true},
        {"denied: idle -> error (not in graph)", "idle", "error", false},
        {"denied: idle -> idle (self-loop not declared)", "idle", "idle", false},
        {"denied: unknown from state", "archived", "idle", false},
        {"denied: unknown to state", "idle", "nonexistent", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := rt.CanTransitionTo(tt.from, tt.to)
            assert.Equal(t, tt.expected, got)
        })
    }
}

func TestValidateMetadata(t *testing.T) {
    rt := &resourcetype.ResourceType{
        JSONSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "serial_number": map[string]interface{}{"type": "string"},
                "diameter_mm":   map[string]interface{}{"type": "number", "minimum": 0},
            },
            "required": []interface{}{"serial_number"},
        },
    }

    t.Run("valid metadata", func(t *testing.T) {
        err := rt.ValidateMetadata(map[string]interface{}{
            "serial_number": "SN-001",
            "diameter_mm":   12.5,
        })
        assert.NoError(t, err)
    })

    t.Run("missing required field", func(t *testing.T) {
        err := rt.ValidateMetadata(map[string]interface{}{
            "diameter_mm": 12.5,
        })
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "serial_number")
    })

    t.Run("wrong type", func(t *testing.T) {
        err := rt.ValidateMetadata(map[string]interface{}{
            "serial_number": 123, // doit etre string
        })
        assert.Error(t, err)
    })

    t.Run("empty schema = no constraint", func(t *testing.T) {
        rtNoSchema := &resourcetype.ResourceType{JSONSchema: map[string]interface{}{}}
        err := rtNoSchema.ValidateMetadata(map[string]interface{}{"anything": true})
        assert.NoError(t, err)
    })
}
```

**Creer** : `kors-api/internal/domain/permission/permission_test.go`

```go
package permission_test

import (
    "testing"
    "time"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
    "github.com/stretchr/testify/assert"
)

func TestPermissionIsExpired(t *testing.T) {
    t.Run("no expiry = never expired", func(t *testing.T) {
        p := &permission.Permission{ID: uuid.New(), ExpiresAt: nil}
        assert.False(t, p.IsExpired())
    })

    t.Run("future expiry = not expired", func(t *testing.T) {
        future := time.Now().Add(time.Hour)
        p := &permission.Permission{ID: uuid.New(), ExpiresAt: &future}
        assert.False(t, p.IsExpired())
    })

    t.Run("past expiry = expired", func(t *testing.T) {
        past := time.Now().Add(-time.Hour)
        p := &permission.Permission{ID: uuid.New(), ExpiresAt: &past}
        assert.True(t, p.IsExpired())
    })
}
```

---

### TACHE 3.3 — Tests des Use Cases (mocks des repositories)

**Creer** : `kors-api/internal/usecase/mocks/mocks.go`

Creer des mocks manuels (ne pas utiliser mockery pour rester sans dependance externe
supplementaire) pour tous les repositories :

```go
package mocks

import (
    "context"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/event"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
    "github.com/haksolot/kors/kors-api/internal/domain/resource"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
)

// --- ResourceTypeRepository mock ---

type ResourceTypeRepo struct {
    Types map[string]*resourcetype.ResourceType
}

func (m *ResourceTypeRepo) Create(_ context.Context, rt *resourcetype.ResourceType) error {
    if m.Types == nil { m.Types = make(map[string]*resourcetype.ResourceType) }
    m.Types[rt.Name] = rt
    return nil
}

func (m *ResourceTypeRepo) GetByName(_ context.Context, name string) (*resourcetype.ResourceType, error) {
    if m.Types == nil { return nil, nil }
    rt, ok := m.Types[name]
    if !ok { return nil, nil }
    return rt, nil
}

func (m *ResourceTypeRepo) GetByID(_ context.Context, id uuid.UUID) (*resourcetype.ResourceType, error) {
    if m.Types == nil { return nil, nil }
    for _, rt := range m.Types {
        if rt.ID == id { return rt, nil }
    }
    return nil, nil
}

func (m *ResourceTypeRepo) List(_ context.Context) ([]*resourcetype.ResourceType, error) {
    result := make([]*resourcetype.ResourceType, 0, len(m.Types))
    for _, rt := range m.Types { result = append(result, rt) }
    return result, nil
}

// --- ResourceRepository mock ---

type ResourceRepo struct {
    Resources map[uuid.UUID]*resource.Resource
    CreateErr error
    UpdateErr error
}

func (m *ResourceRepo) Create(_ context.Context, res *resource.Resource) error {
    if m.CreateErr != nil { return m.CreateErr }
    if m.Resources == nil { m.Resources = make(map[uuid.UUID]*resource.Resource) }
    m.Resources[res.ID] = res
    return nil
}

func (m *ResourceRepo) GetByID(_ context.Context, id uuid.UUID) (*resource.Resource, error) {
    if m.Resources == nil { return nil, nil }
    res, ok := m.Resources[id]
    if !ok { return nil, nil }
    return res, nil
}

func (m *ResourceRepo) Update(_ context.Context, res *resource.Resource) error {
    if m.UpdateErr != nil { return m.UpdateErr }
    if m.Resources == nil { m.Resources = make(map[uuid.UUID]*resource.Resource) }
    m.Resources[res.ID] = res
    return nil
}

func (m *ResourceRepo) List(_ context.Context, first int, after *uuid.UUID, typeName *string) ([]*resource.Resource, bool, int, error) {
    result := make([]*resource.Resource, 0)
    for _, res := range m.Resources { result = append(result, res) }
    return result, false, len(result), nil
}

// --- PermissionRepository mock ---

type PermissionRepo struct {
    // Permet de controler le resultat de Check par action
    Allowed map[string]bool // key: action
    AllowAll bool
}

func (m *PermissionRepo) Create(_ context.Context, p *permission.Permission) error { return nil }
func (m *PermissionRepo) Delete(_ context.Context, id uuid.UUID) error { return nil }
func (m *PermissionRepo) FindForIdentity(_ context.Context, identityID uuid.UUID) ([]*permission.Permission, error) { return nil, nil }

func (m *PermissionRepo) Check(_ context.Context, identityID uuid.UUID, action string, resourceID *uuid.UUID, resourceTypeID *uuid.UUID) (bool, error) {
    if m.AllowAll { return true, nil }
    if m.Allowed != nil {
        return m.Allowed[action], nil
    }
    return false, nil
}

// --- EventRepository mock ---

type EventRepo struct {
    Events []*event.Event
    CreateErr error
}

func (m *EventRepo) Create(_ context.Context, e *event.Event) error {
    if m.CreateErr != nil { return m.CreateErr }
    m.Events = append(m.Events, e)
    return nil
}

// --- EventPublisher mock ---

type EventPublisher struct {
    Published []*event.Event
    PublishErr error
}

func (m *EventPublisher) Publish(_ context.Context, e *event.Event) error {
    if m.PublishErr != nil { return m.PublishErr }
    m.Published = append(m.Published, e)
    return nil
}
```

---

**Creer** : `kors-api/internal/usecase/create_resource_test.go`

```go
package usecase_test

import (
    "context"
    "errors"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
    "github.com/haksolot/kors/kors-api/internal/usecase"
    "github.com/haksolot/kors/kors-api/internal/usecase/mocks"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func sampleResourceType() *resourcetype.ResourceType {
    return &resourcetype.ResourceType{
        ID:   uuid.New(),
        Name: "cnc_machine",
        JSONSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "serial": map[string]interface{}{"type": "string"},
            },
        },
        Transitions: map[string]interface{}{
            "idle":   []interface{}{"in_use"},
            "in_use": []interface{}{"idle"},
        },
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
}

func TestCreateResourceUseCase(t *testing.T) {
    ctx := context.Background()
    callerID := uuid.New()

    t.Run("success", func(t *testing.T) {
        rt := sampleResourceType()
        uc := &usecase.CreateResourceUseCase{
            ResourceTypeRepo: &mocks.ResourceTypeRepo{Types: map[string]*resourcetype.ResourceType{rt.Name: rt}},
            ResourceRepo:     &mocks.ResourceRepo{},
            EventRepo:        &mocks.EventRepo{},
            PermissionRepo:   &mocks.PermissionRepo{AllowAll: true},
            EventPublisher:   &mocks.EventPublisher{},
        }
        res, err := uc.Execute(ctx, usecase.CreateResourceInput{
            TypeName:     rt.Name,
            InitialState: "idle",
            Metadata:     map[string]interface{}{"serial": "SN-001"},
            IdentityID:   callerID,
        })
        require.NoError(t, err)
        assert.NotNil(t, res)
        assert.Equal(t, "idle", res.State)
        assert.Equal(t, rt.ID, res.TypeID)
    })

    t.Run("resource type not found", func(t *testing.T) {
        uc := &usecase.CreateResourceUseCase{
            ResourceTypeRepo: &mocks.ResourceTypeRepo{},
            PermissionRepo:   &mocks.PermissionRepo{AllowAll: true},
        }
        res, err := uc.Execute(ctx, usecase.CreateResourceInput{
            TypeName: "nonexistent", InitialState: "idle", IdentityID: callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, res)
        assert.Contains(t, err.Error(), "not found")
    })

    t.Run("permission denied", func(t *testing.T) {
        rt := sampleResourceType()
        uc := &usecase.CreateResourceUseCase{
            ResourceTypeRepo: &mocks.ResourceTypeRepo{Types: map[string]*resourcetype.ResourceType{rt.Name: rt}},
            PermissionRepo:   &mocks.PermissionRepo{AllowAll: false},
        }
        res, err := uc.Execute(ctx, usecase.CreateResourceInput{
            TypeName: rt.Name, InitialState: "idle", IdentityID: callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, res)
        assert.Contains(t, err.Error(), "permission")
    })

    t.Run("nats failure rolls back (with transaction mock)", func(t *testing.T) {
        rt := sampleResourceType()
        publisher := &mocks.EventPublisher{PublishErr: errors.New("nats down")}
        uc := &usecase.CreateResourceUseCase{
            ResourceTypeRepo: &mocks.ResourceTypeRepo{Types: map[string]*resourcetype.ResourceType{rt.Name: rt}},
            ResourceRepo:     &mocks.ResourceRepo{},
            EventRepo:        &mocks.EventRepo{},
            PermissionRepo:   &mocks.PermissionRepo{AllowAll: true},
            EventPublisher:   publisher,
        }
        res, err := uc.Execute(ctx, usecase.CreateResourceInput{
            TypeName: rt.Name, InitialState: "idle", IdentityID: callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, res)
        assert.Contains(t, err.Error(), "bus error")
    })

    t.Run("metadata schema validation fails", func(t *testing.T) {
        rt := sampleResourceType()
        // serial doit etre une string, on envoie un int
        uc := &usecase.CreateResourceUseCase{
            ResourceTypeRepo: &mocks.ResourceTypeRepo{Types: map[string]*resourcetype.ResourceType{rt.Name: rt}},
            PermissionRepo:   &mocks.PermissionRepo{AllowAll: true},
        }
        res, err := uc.Execute(ctx, usecase.CreateResourceInput{
            TypeName:     rt.Name,
            InitialState: "idle",
            Metadata:     map[string]interface{}{"serial": 999},
            IdentityID:   callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, res)
        assert.Contains(t, err.Error(), "metadata")
    })
}
```

---

**Creer** : `kors-api/internal/usecase/transition_resource_test.go`

Meme pattern que `create_resource_test.go`. Couvrir :
- Cas nominal (transition autorisee)
- Resource introuvable
- Permission refusee
- Transition interdite par la FSM (ex: `idle -> error` si non declare)
- Mise a jour des metadata lors de la transition
- Emission de l'evenement `kors.resource.state_changed`

---

**Creer** : `kors-api/internal/usecase/register_resource_type_test.go`

Couvrir :
- Cas nominal
- Nom vide -> erreur
- Permission non-admin -> erreur
- Erreur de persistance

---

### TACHE 3.4 — Tests d'integration des repositories Postgres

**Creer** : `kors-api/internal/adapter/postgres/resource_repo_test.go`

Utiliser `testhelper.SetupTestDB` et tester :

```go
package postgres_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/adapter/postgres"
    "github.com/haksolot/kors/kors-api/internal/domain/resource"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
    "github.com/haksolot/kors/kors-api/internal/testhelper"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestResourceRepository(t *testing.T) {
    pool := testhelper.SetupTestDB(t)
    ctx := context.Background()

    rtRepo := &postgres.ResourceTypeRepository{Pool: pool}
    rRepo := &postgres.ResourceRepository{Pool: pool}

    // Setup: creer un type de resource
    rt := &resourcetype.ResourceType{
        ID: uuid.New(), Name: "test_type",
        JSONSchema: map[string]interface{}{"type": "object"},
        Transitions: map[string]interface{}{"idle": []interface{}{"in_use"}},
        CreatedAt: time.Now().Truncate(time.Microsecond),
        UpdatedAt: time.Now().Truncate(time.Microsecond),
    }
    require.NoError(t, rtRepo.Create(ctx, rt))

    t.Run("Create and GetByID", func(t *testing.T) {
        res := &resource.Resource{
            ID: uuid.New(), TypeID: rt.ID, State: "idle",
            Metadata: map[string]interface{}{"key": "value"},
            CreatedAt: time.Now().Truncate(time.Microsecond),
            UpdatedAt: time.Now().Truncate(time.Microsecond),
        }
        require.NoError(t, rRepo.Create(ctx, res))

        found, err := rRepo.GetByID(ctx, res.ID)
        require.NoError(t, err)
        require.NotNil(t, found)
        assert.Equal(t, res.ID, found.ID)
        assert.Equal(t, "idle", found.State)
    })

    t.Run("GetByID not found returns nil", func(t *testing.T) {
        found, err := rRepo.GetByID(ctx, uuid.New())
        assert.NoError(t, err)
        assert.Nil(t, found)
    })

    t.Run("Update state", func(t *testing.T) {
        res := &resource.Resource{
            ID: uuid.New(), TypeID: rt.ID, State: "idle",
            Metadata: map[string]interface{}{},
            CreatedAt: time.Now().Truncate(time.Microsecond),
            UpdatedAt: time.Now().Truncate(time.Microsecond),
        }
        require.NoError(t, rRepo.Create(ctx, res))
        res.State = "in_use"
        res.UpdatedAt = time.Now().Truncate(time.Microsecond)
        require.NoError(t, rRepo.Update(ctx, res))

        found, _ := rRepo.GetByID(ctx, res.ID)
        assert.Equal(t, "in_use", found.State)
    })

    t.Run("List with pagination", func(t *testing.T) {
        // Creer 5 resources
        for i := 0; i < 5; i++ {
            r := &resource.Resource{
                ID: uuid.New(), TypeID: rt.ID, State: "idle",
                Metadata: map[string]interface{}{},
                CreatedAt: time.Now().Truncate(time.Microsecond),
                UpdatedAt: time.Now().Truncate(time.Microsecond),
            }
            require.NoError(t, rRepo.Create(ctx, r))
        }
        results, hasNext, total, err := rRepo.List(ctx, 3, nil, nil)
        require.NoError(t, err)
        assert.Len(t, results, 3)
        assert.True(t, hasNext)
        assert.GreaterOrEqual(t, total, 5)
    })

    t.Run("List filter by typeName", func(t *testing.T) {
        typeName := "test_type"
        results, _, _, err := rRepo.List(ctx, 100, nil, &typeName)
        require.NoError(t, err)
        for _, r := range results {
            assert.Equal(t, rt.ID, r.TypeID)
        }
    })
}
```

**Creer** : `kors-api/internal/adapter/postgres/permission_repo_test.go`

Tester :
- `Create` et `Check` pour les 3 niveaux de scope (global, par type, par resource)
- `Check` avec une permission expiree -> doit retourner false
- `Check` avec une permission non expiree -> doit retourner true
- `FindForIdentity` filtre les permissions expirees
- `Delete` supprime la permission

---

### TACHE 3.5 — Tests du worker et du consommateur NATS

**Creer** : `kors-worker/internal/jobs/permission_cleanup_test.go`

Ce test verifie le comportement du job sans necessite de NATS.
Utiliser un mock du `PermissionRepository` du worker.
Tester :
- Comportement nominal : `CleanupExpired` est appele et retourne un count > 0
- Comportement avec verrou pris : `ErrLocked` est retourne, le job skip silencieusement
- Comportement avec erreur generique : l'erreur est loggee, le job continue

---

## PRIORITE 4 — Fonctionnalites manquantes

### TACHE 4.1 — Cache des identites dans le middleware Auth

**Probleme** : Chaque requete GraphQL fait un `SELECT` en DB pour retrouver l'identite via
son `external_id`. Avec N instances de kors-api sous charge, c'est un hot path.

**Ce que tu dois faire** :

1. Creer `kors-api/internal/middleware/identity_cache.go` :

```go
package middleware

import (
    "sync"
    "time"
    "github.com/google/uuid"
)

type cachedIdentity struct {
    id        uuid.UUID
    expiresAt time.Time
}

type IdentityCache struct {
    mu    sync.RWMutex
    cache map[string]cachedIdentity
    ttl   time.Duration
}

func NewIdentityCache(ttl time.Duration) *IdentityCache {
    c := &IdentityCache{cache: make(map[string]cachedIdentity), ttl: ttl}
    go c.evict()
    return c
}

func (c *IdentityCache) Get(externalID string) (uuid.UUID, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    entry, ok := c.cache[externalID]
    if !ok || time.Now().After(entry.expiresAt) {
        return uuid.Nil, false
    }
    return entry.id, true
}

func (c *IdentityCache) Set(externalID string, id uuid.UUID) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.cache[externalID] = cachedIdentity{id: id, expiresAt: time.Now().Add(c.ttl)}
}

func (c *IdentityCache) evict() {
    ticker := time.NewTicker(c.ttl)
    for range ticker.C {
        c.mu.Lock()
        for k, v := range c.cache {
            if time.Now().After(v.expiresAt) {
                delete(c.cache, k)
            }
        }
        c.mu.Unlock()
    }
}
```

2. Ajouter `IdentityCache *IdentityCache` dans `AuthMiddleware` et l'utiliser avant le lookup DB :

```go
if c, ok := m.IdentityCache.Get(externalID); ok {
    ctx := korsctx.WithIdentity(r.Context(), c)
    next.ServeHTTP(w, r.WithContext(ctx))
    return
}
// Sinon: lookup DB puis Set dans le cache
```

3. Configurer via `IDENTITY_CACHE_TTL` (defaut : `5m`).

---

### TACHE 4.2 — Endpoint /health enrichi

**Fichier** : `kors-api/cmd/server/main.go`

**Probleme actuel** : `/healthz` retourne juste "OK" sans verifier les dependances.

**Ce que tu dois faire** :

Remplacer `HealthCheck` par un handler qui verifie DB et NATS :

```go
type HealthStatus struct {
    Status   string            `json:"status"`
    Checks   map[string]string `json:"checks"`
    Hostname string            `json:"hostname"`
}

func makeHealthHandler(pool *pgxpool.Pool, nc *nats.Conn) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        checks := make(map[string]string)
        overall := "ok"

        // DB check
        if err := pool.Ping(r.Context()); err != nil {
            checks["database"] = "error: " + err.Error()
            overall = "degraded"
        } else {
            checks["database"] = "ok"
        }

        // NATS check
        if nc == nil || !nc.IsConnected() {
            checks["nats"] = "disconnected"
            overall = "degraded"
        } else {
            checks["nats"] = "ok"
        }

        hostname, _ := os.Hostname()
        status := HealthStatus{Status: overall, Checks: checks, Hostname: hostname}

        w.Header().Set("Content-Type", "application/json")
        if overall != "ok" {
            w.WriteHeader(http.StatusServiceUnavailable)
        }
        json.NewEncoder(w).Encode(status)
    }
}
```

Enregistrer : `mux.Get("/healthz", makeHealthHandler(pool, nc))`

---

### TACHE 4.3 — Reaction utile dans kors-events

**Fichier** : `kors-events/internal/adapter/nats/consumer.go`

**Probleme actuel** : Le consommateur confirme les messages mais ne fait rien d'utile.

**Ce que tu dois faire** :

1. Creer une interface `EventHandler` dans `kors-events/internal/domain/event/` :

```go
type Handler interface {
    Handle(ctx context.Context, subject string, payload []byte) error
}
```

2. Creer `kors-events/internal/handler/log_handler.go` comme handler de base qui logue
   les evenements recus avec leur type et resource_id, puis creer l'infrastructure pour
   brancher des handlers additionnels facilement.

3. Faire passer la liste de handlers a `EventConsumer` :

```go
type EventConsumer struct {
    JS       nats.JetStreamContext
    EventRepo event.Repository
    Handlers  []event.Handler
}
```

4. Dans `handleMessage`, apres le `msg.Ack()`, appeler chaque handler en ordre :

```go
for _, h := range c.Handlers {
    if err := h.Handle(ctx, msg.Subject, msg.Data); err != nil {
        log.Printf("handler error for subject %s: %v", msg.Subject, err)
    }
}
```

---

### TACHE 4.4 — Pagination EventConnection et RevisionConnection

**Fichiers** :
- `kors-api/internal/graph/schema/schema.graphql`
- `kors-api/internal/domain/revision/revision.go`
- `kors-api/internal/adapter/postgres/revision_repo.go`

**Ce que tu dois faire** :

1. Ajouter dans `schema.graphql` :

```graphql
type EventConnection {
  edges: [EventEdge!]!
  pageInfo: PageInfo!
  totalCount: Int!
}

type EventEdge {
  cursor: String!
  node: Event!
}

type RevisionConnection {
  edges: [RevisionEdge!]!
  pageInfo: PageInfo!
  totalCount: Int!
}

type RevisionEdge {
  cursor: String!
  node: Revision!
}

# Sur le type Resource, remplacer :
# revisions: [Revision!]!
# Par :
revisions(first: Int, after: String): RevisionConnection!
```

2. Ajouter `ListByResourcePaginated` sur `revision.Repository` et implementer dans le repo postgres.

---

## PRIORITE 5 — Qualite du code

### TACHE 5.1 — Remplacer fmt.Printf par un logger structure

**Probleme** :
Plusieurs endroits utilisent `fmt.Printf` pour les warnings au lieu d'un logger structure :
- `kors-api/internal/usecase/transition_resource.go` : `fmt.Printf("Warning: ...")`
- `kors-events/internal/adapter/nats/consumer.go` : `log.Printf`
- `kors-worker/internal/jobs/permission_cleanup.go` : `log.Printf`

**Ce que tu dois faire** :

1. Ajouter `github.com/rs/zerolog` dans les `go.mod` de `kors-api`, `kors-events`, `kors-worker`.

2. Creer `shared/logger/logger.go` :

```go
package logger

import (
    "os"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func Init(serviceName string) zerolog.Logger {
    level := zerolog.InfoLevel
    if os.Getenv("APP_ENV") == "development" {
        level = zerolog.DebugLevel
        log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
    }
    zerolog.SetGlobalLevel(level)
    return log.With().Str("service", serviceName).Logger()
}
```

3. Remplacer tous les `fmt.Printf` et `log.Printf` par des appels zerolog structures.
   En particulier dans `transition_resource.go`, passer le logger en dependance du UseCase.

---

### TACHE 5.2 — Validation des actions RBAC

**Fichier** : `kors-api/internal/usecase/grant_permission.go`

**Probleme actuel** :
Aucune validation que `input.Action` est une valeur parmi `read`, `write`, `transition`, `admin`.
N'importe quelle chaine peut etre inseree en DB.

**Ce que tu dois faire** :

Ajouter au debut de `GrantPermissionUseCase.Execute` :

```go
var validActions = map[string]bool{
    "read": true, "write": true, "transition": true, "admin": true,
}

func (uc *GrantPermissionUseCase) Execute(ctx context.Context, input GrantPermissionInput) (*permission.Permission, error) {
    if !validActions[input.Action] {
        return nil, fmt.Errorf("invalid action %q: must be one of read, write, transition, admin", input.Action)
    }
    // Interdire d'accorder admin sans etre soi-meme admin
    // (a implementer : verifier que le caller a 'admin')
    // ...
}
```

---

### TACHE 5.3 — Taille maximale des uploads

**Fichier** : `kors-api/internal/usecase/upload_file.go`

**Probleme actuel** : Aucune limite sur la taille du fichier uploade en base64.

Ajouter une constante et une verification :

```go
const maxUploadSizeBytes = 50 * 1024 * 1024 // 50 MB

func (uc *UploadFileUseCase) Execute(ctx context.Context, input UploadFileInput) (*UploadFileOutput, error) {
    // ...
    content, err := base64.StdEncoding.DecodeString(input.FileContent)
    if err != nil {
        return &UploadFileOutput{Success: false, Error: "invalid base64 content"}, nil
    }
    if len(content) > maxUploadSizeBytes {
        return &UploadFileOutput{Success: false, Error: fmt.Sprintf("file too large: max %d MB", maxUploadSizeBytes/1024/1024)}, nil
    }
    // ...
}
```

---

### TACHE 5.4 — Nettoyage du wiring dans cmd/server/main.go

**Fichier** : `kors-api/cmd/server/main.go`

**Probleme actuel** : `main()` fait plus de 120 lignes et mixe configuration, initialisation
des dependances, bootstrapping et routage HTTP.

**Ce que tu dois faire** :

Extraire dans des fonctions separees dans le meme package :

```go
// config.go : lecture de toutes les variables d'environnement dans une struct Config
type Config struct {
    Port                 string
    DatabaseURL          string
    NatsURL              string
    MinioURL             string
    MinioAccessKey       string
    MinioSecretKey       string
    MinioBucket          string
    ComplexityLimit      int
    GraphQLIntrospection bool
    JWKSEndpoint         string
    AppEnv               string
    OTLPEndpoint         string
    OTLPInsecure         bool
}

func loadConfig() Config { ... }

// bootstrap.go : fonctions pour initialiser chaque dependance
func connectDB(ctx context.Context, url string) (*pgxpool.Pool, error) { ... }
func connectNATS(url string) (*nats.Conn, nats.JetStreamContext, error) { ... }
func connectMinio(url, ak, sk string, ssl bool) (*minio.Client, error) { ... }
func bootstrapSystemIdentity(ctx context.Context, idRepo identity.Repository, pRepo permission.Repository) error { ... }
```

---

## TACHES TRANSVERSES

### Mise a jour de .env.example

Ajouter toutes les variables manquantes :

```env
# Application
APP_ENV=development
PORT=8080

# Database
DATABASE_URL=postgres://kors:kors_dev_secret@localhost:5432/kors?sslmode=disable

# NATS
NATS_URL=nats://localhost:4222

# MinIO
MINIO_URL=localhost:9000
MINIO_ACCESS_KEY=kors_admin
MINIO_SECRET_KEY=kors_dev_secret
MINIO_BUCKET=kors-files
MINIO_USE_SSL=false

# Keycloak / Auth
JWKS_ENDPOINT=http://localhost:8180/realms/kors/protocol/openid-connect/certs

# Observabilite
OTLP_ENDPOINT=localhost:4317
OTLP_INSECURE=true

# GraphQL
GRAPHQL_COMPLEXITY_LIMIT=1000
GRAPHQL_INTROSPECTION=false

# Worker
PERMISSIONS_CLEANUP_INTERVAL=1h
EVENTS_ARCHIVE_AFTER=2160h

# Cache
IDENTITY_CACHE_TTL=5m
```

---

### Makefile — Cibles de test a ajouter

Dans `Makefile`, ajouter :

```makefile
.PHONY: test test-unit test-integration lint

test-unit:
	cd kors-api && go test ./internal/domain/... ./internal/usecase/... -v -count=1

test-integration:
	cd kors-api && go test ./internal/adapter/postgres/... -v -count=1 -timeout 120s

test:
	make test-unit
	make test-integration
	cd kors-events && go test ./... -v -count=1
	cd kors-worker && go test ./... -v -count=1

lint:
	cd kors-api && go vet ./...
	cd kors-events && go vet ./...
	cd kors-worker && go vet ./...
```

---

## Instructions finales pour l'agent

1. **Ordre d'execution recommande** : Taches 1.2, 1.3, 2.1, 2.2 d'abord (elles modifient des
   fichiers dont d'autres taches dependent). Puis 3.1 (testhelper) avant tous les autres tests.
   Puis 1.1 (JWT) car elle necessite des tests pour etre validee.

2. **Apres chaque modification de schema GraphQL** (`schema.graphql`), executer :
   ```
   cd kors-api && go run github.com/99designs/gqlgen generate
   ```
   Puis implementer les nouveaux resolvers generes dans `schema.resolvers.go`.

3. **Verifier la compilation** apres chaque tache :
   ```
   cd kors-api && go build ./...
   cd kors-events && go build ./...
   cd kors-worker && go build ./...
   ```

4. **Ne pas modifier** les fichiers generes par gqlgen dans `internal/graph/generated/`.
   Ces fichiers sont ecrases a chaque `go generate`.

5. **Ne pas modifier** `go.work` ni les `go.sum` manuellement. Utiliser `go get` et `go mod tidy`.

6. **Imports** : Utiliser le module path `github.com/haksolot/kors/kors-api` pour tous les imports
   internes a `kors-api`. Le workspace Go (`go.work`) gere la resolution entre modules.
