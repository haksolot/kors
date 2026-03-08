# GEMINI.md — Feuille de route KORS v2

Tu as acces a l'integralite de la codebase KORS. Ce fichier est ta source de verite unique.
Lis-le entierement avant de commencer. Les taches sont ordonnees par dependance : certaines
sont des prerequis d'autres. Respecte cet ordre ou tu casseras le wiring.

---

## Rappel d'architecture

Module Go racine : `github.com/haksolot/kors/kors-api`
Workspace Go : `go.work` a la racine, gere les remplacements entre modules.

Apres toute modification de `schema.graphql`, regenerer gqlgen :

```
cd kors-api && go run github.com/99designs/gqlgen generate
```

Verifier la compilation apres chaque tache :

```
cd kors-api && go build ./...
```

Ne jamais modifier les fichiers dans `internal/graph/generated/` — ils sont ecrases par gqlgen.

---

## SPRINT 1 — Debloquer les modules (prerequis a tout le reste)

Ces taches resolvent les deux blocages fonctionnels critiques : un module provisionne ne peut
actuellement rien faire via l'API KORS sans intervention manuelle d'un admin, et les credentials
qu'il recoit sont incomplets pour s'auto-configurer.

---

### TACHE 1.1 — Accorder automatiquement les permissions KORS au module lors du provisionnement

**Probleme identifie :** Apres `provisionModule`, le module a un schema Postgres et un bucket
MinIO, mais son identite KORS (`type: "service"`) n'a aucune entree dans `kors.permissions`.
Tout appel a `createResource`, `transitionResource` ou `createRevision` sera refuse par le
RBAC. Le module est provisionne mais inutilisable sans appel manuel a `grantPermission`.

**Fichier :** `kors-api/internal/usecase/provision_module.go`

Ajouter une methode privee `grantModulePermissions` dans `ModuleGovernanceUseCase` :

```go
func (uc *ModuleGovernanceUseCase) grantModulePermissions(ctx context.Context, identID uuid.UUID) error {
    for _, action := range []string{"read", "write", "transition"} {
        err := uc.PermissionRepo.Create(ctx, &permission.Permission{
            ID:         uuid.New(),
            IdentityID: identID,
            // ResourceID et ResourceTypeID sont nil => scope global
            Action:    action,
            CreatedAt: time.Now(),
        })
        if err != nil {
            return fmt.Errorf("failed to grant %s permission to module: %w", action, err)
        }
    }
    return nil
}
```

Le module recoit `read`, `write` et `transition` en scope global. Il ne recoit PAS `admin`
— ce droit reste reserve aux operateurs humains.

Reecrire `Provision` pour appeler cette methode apres la creation de l'identite :

```go
func (uc *ModuleGovernanceUseCase) Provision(ctx context.Context, moduleName string, identityID uuid.UUID) (*provisioning.ModuleCredentials, error) {
    if err := validateModuleName(moduleName); err != nil { return nil, err }
    if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }

    // 1. Creer ou recuperer l'identite KORS du module
    ident, err := uc.IdentityRepo.GetByExternalID(ctx, moduleName)
    if err != nil { return nil, err }
    if ident == nil {
        newIdent := &identity.Identity{
            ID:         uuid.New(),
            ExternalID: moduleName,
            Name:       moduleName,
            Type:       "service",
            CreatedAt:  time.Now(),
            UpdatedAt:  time.Now(),
        }
        if err := uc.IdentityRepo.Create(ctx, newIdent); err != nil {
            return nil, fmt.Errorf("failed to create module identity: %w", err)
        }
        ident = newIdent
    }

    // 2. Accorder les permissions KORS au module
    if err := uc.grantModulePermissions(ctx, ident.ID); err != nil {
        return nil, err
    }

    // 3. Provisionner MinIO
    if uc.StorageProvisioner != nil {
        if err := uc.StorageProvisioner.ProvisionBucket(ctx, moduleName); err != nil {
            return nil, fmt.Errorf("failed to provision storage: %w", err)
        }
    }

    // 4. Provisionner Postgres
    return uc.Provisioner.ProvisionModule(ctx, moduleName)
}
```

---

### TACHE 1.2 — Abaisser la permission requise pour registerResourceType

**Probleme identifie :** `registerResourceType` exige `admin` global. Un module n'a que
`write` global apres provisionnement. Il ne peut donc pas declarer ses propres types metier.

**Fichier :** `kors-api/internal/usecase/register_resource_type.go`

Remplacer le bloc de check permission :

```go
// Avant
allowed, err := uc.PermissionRepo.Check(ctx, input.IdentityID, "admin", nil, nil)
if !allowed { return nil, fmt.Errorf("...admin role required...") }

// Apres : write global OU admin global suffisent
allowedWrite, err := uc.PermissionRepo.Check(ctx, input.IdentityID, "write", nil, nil)
if err != nil { return nil, fmt.Errorf("failed to check permission: %w", err) }
allowedAdmin, err := uc.PermissionRepo.Check(ctx, input.IdentityID, "admin", nil, nil)
if err != nil { return nil, fmt.Errorf("failed to check permission: %w", err) }
if !allowedWrite && !allowedAdmin {
    return nil, fmt.Errorf("identity %s is not authorized to register types (write or admin required)", input.IdentityID)
}
```

---

### TACHE 1.3 — Abaisser la permission requise pour createIdentity

**Probleme identifie :** `createIdentity` exige `admin` global. Un module qui veut enregistrer
des identites utilisateurs (ex: les operateurs d'une machine) ne peut pas le faire.

**Fichier :** `kors-api/internal/usecase/create_identity.go`

Remplacer le bloc de check permission par une logique granulaire :

```go
isAdmin, err := uc.PermissionRepo.Check(ctx, input.CallerID, "admin", nil, nil)
if err != nil { return nil, fmt.Errorf("failed to check permission: %w", err) }
hasWrite, err := uc.PermissionRepo.Check(ctx, input.CallerID, "write", nil, nil)
if err != nil { return nil, fmt.Errorf("failed to check permission: %w", err) }

switch input.Type {
case "user":
    // write ou admin suffisent pour creer un utilisateur
    if !hasWrite && !isAdmin {
        return nil, fmt.Errorf("write or admin permission required to create user identities")
    }
case "service", "system":
    // seul admin peut creer des identites service ou system
    if !isAdmin {
        return nil, fmt.Errorf("admin permission required to create %s identities", input.Type)
    }
default:
    return nil, fmt.Errorf("invalid identity type %q: must be one of user, service, system", input.Type)
}
```

Supprimer la validation de type qui existait apres (elle est dans le switch ci-dessus).

---

### TACHE 1.4 — Enrichir ProvisioningResult avec connectionString et bucketName

**Probleme identifie :** Le module recoit `username` et `password` separes mais doit
assembler lui-meme l'URL de connexion Postgres. Il ne connait pas non plus le nom exact
de son bucket MinIO. Il ne peut pas s'auto-configurer.

**Fichier 1 :** `kors-api/internal/domain/provisioning/provisioning.go`

Ajouter les champs dans `ModuleCredentials` :

```go
type ModuleCredentials struct {
    ModuleName       string
    Schema           string
    Username         string
    Password         string
    ConnectionString string // ex: postgres://user_tms:xxx@host/db?options=-csearch_path%3Dtms%2Ckors
    BucketName       string // ex: module-tms
}
```

**Fichier 2 :** `kors-api/internal/adapter/postgres/provisioner.go`

Ajouter une fonction helper et l'utiliser dans `ProvisionModule` :

```go
import (
    "net/url"
    "os"
    "strings"
)

func buildConnectionString(username, password, schema string) string {
    base := os.Getenv("DATABASE_URL")
    u, err := url.Parse(base)
    if err != nil { return "" }
    u.User = url.UserPassword(username, password)
    q := u.Query()
    q.Set("options", fmt.Sprintf("-csearch_path=%s,kors", schema))
    u.RawQuery = q.Encode()
    return u.String()
}
```

Dans la valeur de retour de `ProvisionModule`, construire les deux nouveaux champs :

```go
bucketName := fmt.Sprintf("module-%s", strings.ReplaceAll(strings.ToLower(moduleName), "_", "-"))
return &provisioning.ModuleCredentials{
    ModuleName:       moduleName,
    Schema:           schema,
    Username:         username,
    Password:         password,
    ConnectionString: buildConnectionString(username, password, schema),
    BucketName:       bucketName,
}, nil
```

**Fichier 3 :** `kors-api/internal/graph/schema/schema.graphql`

```graphql
type ProvisioningResult {
  success: Boolean!
  moduleName: String
  schema: String
  username: String
  password: String
  connectionString: String
  bucketName: String
  error: MutationError
}
```

**Fichier 4 :** `kors-api/internal/graph/resolvers/schema.resolvers.go`

Ajouter les deux champs dans le retour du resolver `ProvisionModule` :

```go
return &model.ProvisioningResult{
    Success:          true,
    ModuleName:       &creds.ModuleName,
    Schema:           &creds.Schema,
    Username:         &creds.Username,
    Password:         &creds.Password,
    ConnectionString: &creds.ConnectionString,
    BucketName:       &creds.BucketName,
}, nil
```

Regenerer gqlgen apres modification du schema.

---

### TACHE 1.5 — Configurer le search_path PostgreSQL du role module

**Probleme identifie :** Sans `search_path`, le module doit prefixer toutes ses tables avec
`tms.` dans ses requetes SQL. Avec le search_path, il ecrit directement `tools` et a aussi
acces transparent aux tables `kors.*` en lecture.

**Fichier :** `kors-api/internal/adapter/postgres/provisioner.go`

Dans la liste `queries` de `ProvisionModule`, inserer cette ligne avant `ALTER SCHEMA OWNER` :

```go
fmt.Sprintf("ALTER ROLE %s SET search_path TO %s, kors", username, schema),
```

Liste finale des queries dans l'ordre :

```
CREATE SCHEMA IF NOT EXISTS <schema>
DROP ROLE IF EXISTS <username>
CREATE ROLE <username> WITH LOGIN PASSWORD '<password>'
GRANT USAGE ON SCHEMA kors TO <username>
GRANT SELECT ON ALL TABLES IN SCHEMA kors TO <username>
ALTER DEFAULT PRIVILEGES IN SCHEMA kors GRANT SELECT ON TABLES TO <username>
ALTER ROLE <username> SET search_path TO <schema>, kors    <- NOUVEAU
ALTER SCHEMA <schema> OWNER TO <username>
```

---

## SPRINT 2 — Registre des modules et administration complete

---

### TACHE 2.1 — Creer la table kors.modules comme source de verite

**Probleme identifie :** `ListModules` interroge `pg_roles` avec `LIKE 'user_%'`. La liste
des modules est fragile, sans traçabilite (qui a provisionne, quand), et peut retourner des
faux positifs si d'autres roles Postgres suivent la meme convention de nommage.

**Fichier 1 :** `kors-api/migrations/00002_add_modules_registry.sql` (nouveau fichier)

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE kors.modules (
    name           TEXT PRIMARY KEY,
    schema_name    TEXT NOT NULL,
    pg_username    TEXT NOT NULL,
    minio_bucket   TEXT NOT NULL,
    identity_id    UUID REFERENCES kors.identities(id) ON DELETE SET NULL,
    provisioned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    provisioned_by UUID REFERENCES kors.identities(id) ON DELETE SET NULL
);

CREATE INDEX idx_modules_identity ON kors.modules(identity_id);

-- Index de performance pour les queries soft-delete (si absent de la migration 1)
CREATE INDEX IF NOT EXISTS idx_resources_deleted_at
    ON kors.resources(deleted_at) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS kors.modules;
-- +goose StatementEnd
```

**Fichier 2 :** `kors-api/internal/domain/provisioning/provisioning.go`

Ajouter `ModuleRecord` et etendre l'interface `Service` :

```go
type ModuleRecord struct {
    Name          string
    SchemaName    string
    PgUsername    string
    MinioBucket   string
    IdentityID    *uuid.UUID
    ProvisionedAt time.Time
    ProvisionedBy *uuid.UUID
}

type Service interface {
    ProvisionModule(ctx context.Context, moduleName string) (*ModuleCredentials, error)
    DeprovisionModule(ctx context.Context, moduleName string) error
    ListModules(ctx context.Context) ([]string, error)
    RegisterModule(ctx context.Context, record *ModuleRecord) error          // NOUVEAU
    UnregisterModule(ctx context.Context, moduleName string) error           // NOUVEAU
    GetModule(ctx context.Context, moduleName string) (*ModuleRecord, error) // NOUVEAU
    RotatePassword(ctx context.Context, moduleName string) (*ModuleCredentials, error) // NOUVEAU (tache 2.4)
}
```

**Fichier 3 :** `kors-api/internal/adapter/postgres/provisioner.go`

Implementer les trois nouvelles methodes et remplacer `ListModules` :

```go
func (p *PostgresProvisioner) RegisterModule(ctx context.Context, r *provisioning.ModuleRecord) error {
    _, err := p.Pool.Exec(ctx, `
        INSERT INTO kors.modules
            (name, schema_name, pg_username, minio_bucket, identity_id, provisioned_at, provisioned_by)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (name) DO UPDATE SET
            schema_name    = EXCLUDED.schema_name,
            pg_username    = EXCLUDED.pg_username,
            minio_bucket   = EXCLUDED.minio_bucket,
            identity_id    = EXCLUDED.identity_id,
            provisioned_at = EXCLUDED.provisioned_at,
            provisioned_by = EXCLUDED.provisioned_by
    `, r.Name, r.SchemaName, r.PgUsername, r.MinioBucket,
        r.IdentityID, r.ProvisionedAt, r.ProvisionedBy)
    return err
}

func (p *PostgresProvisioner) UnregisterModule(ctx context.Context, moduleName string) error {
    _, err := p.Pool.Exec(ctx, "DELETE FROM kors.modules WHERE name = $1", moduleName)
    return err
}

func (p *PostgresProvisioner) GetModule(ctx context.Context, moduleName string) (*provisioning.ModuleRecord, error) {
    var r provisioning.ModuleRecord
    err := p.Pool.QueryRow(ctx, `
        SELECT name, schema_name, pg_username, minio_bucket, identity_id, provisioned_at, provisioned_by
        FROM kors.modules WHERE name = $1
    `, moduleName).Scan(&r.Name, &r.SchemaName, &r.PgUsername, &r.MinioBucket,
        &r.IdentityID, &r.ProvisionedAt, &r.ProvisionedBy)
    if err != nil {
        if err == pgx.ErrNoRows { return nil, nil }
        return nil, err
    }
    return &r, nil
}

// Remplacer l'implementation actuelle basee sur pg_roles
func (p *PostgresProvisioner) ListModules(ctx context.Context) ([]string, error) {
    rows, err := p.Pool.Query(ctx, "SELECT name FROM kors.modules ORDER BY provisioned_at DESC")
    if err != nil { return nil, err }
    defer rows.Close()
    var names []string
    for rows.Next() {
        var name string
        if err := rows.Scan(&name); err != nil { return nil, err }
        names = append(names, name)
    }
    return names, nil
}
```

**Fichier 4 :** `kors-api/internal/usecase/provision_module.go`

Dans `Provision`, appeler `RegisterModule` apres le succes du provisionnement Postgres :

```go
creds, err := uc.Provisioner.ProvisionModule(ctx, moduleName)
if err != nil { return nil, err }

identIDPtr := &ident.ID
callerIDPtr := &identityID
_ = uc.Provisioner.RegisterModule(ctx, &provisioning.ModuleRecord{
    Name:          moduleName,
    SchemaName:    creds.Schema,
    PgUsername:    creds.Username,
    MinioBucket:   creds.BucketName,
    IdentityID:    identIDPtr,
    ProvisionedAt: time.Now(),
    ProvisionedBy: callerIDPtr,
})

return creds, nil
```

Dans `Deprovision`, appeler `UnregisterModule` en fin de sequence (voir tache 2.3).

---

### TACHE 2.2 — Type ModuleInfo et queries d'administration

**Probleme identifie :** `provisionedModules` retourne une `[String!]!`. L'admin ne voit
que les noms, sans details (bucket, identite, date, schema). Il n'existe pas de query
pour obtenir les details d'un module specifique.

**Fichier 1 :** `kors-api/internal/graph/schema/schema.graphql`

Ajouter le type et modifier les queries :

```graphql
type ModuleInfo {
  name: String!
  schemaName: String!
  pgUsername: String!
  bucketName: String!
  identity: Identity
  provisionedAt: DateTime!
}

# Modifier provisionedModules pour retourner ModuleInfo
# Ajouter une query module(name)
type Query {
  # ... queries existantes (resource, resources, resourceType, resourceTypes) ...

  """
  Liste tous les modules provisiones avec leurs details. Admin requis.
  """
  provisionedModules: [ModuleInfo!]!

  """
  Retourne les details d'un module specifique. Admin requis.
  """
  module(name: String!): ModuleInfo
}
```

**Fichier 2 :** `kors-api/internal/usecase/provision_module.go`

Ajouter deux methodes sur `ModuleGovernanceUseCase` :

```go
func (uc *ModuleGovernanceUseCase) ListDetailed(ctx context.Context, identityID uuid.UUID) ([]*provisioning.ModuleRecord, error) {
    if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }
    names, err := uc.Provisioner.ListModules(ctx)
    if err != nil { return nil, err }
    records := make([]*provisioning.ModuleRecord, 0, len(names))
    for _, name := range names {
        r, err := uc.Provisioner.GetModule(ctx, name)
        if err == nil && r != nil {
            records = append(records, r)
        }
    }
    return records, nil
}

func (uc *ModuleGovernanceUseCase) GetByName(ctx context.Context, moduleName string, identityID uuid.UUID) (*provisioning.ModuleRecord, error) {
    if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }
    return uc.Provisioner.GetModule(ctx, moduleName)
}
```

**Fichier 3 :** `kors-api/internal/graph/resolvers/schema.resolvers.go`

Implementer les deux resolvers. Pour mapper `*provisioning.ModuleRecord` vers `*model.ModuleInfo`,
charger l'identite depuis `IdentityRepo` si `record.IdentityID != nil`.

**Fichier 4 :** `kors-api/internal/graph/resolvers/resolver.go`

S'assurer que `IdentityRepo` est accessible dans `Resolver` pour les resolvers de modules
(passer `idRepo` depuis `NewResolver` si pas deja fait).

Regenerer gqlgen apres modification du schema.

---

### TACHE 2.3 — Deprovision complet avec nettoyage KORS

**Probleme identifie :** `deprovisionModule` supprime schema Postgres et bucket MinIO mais
laisse dans KORS : l'identite du module, ses permissions, et des `kors.resources` orphelines.
Les erreurs de suppression du bucket sont silencieuses (`_ = p.Client.RemoveBucket(...)`).
La mutation retourne `Boolean!` sans distinction d'erreur.

**Fichier 1 :** `kors-api/internal/graph/schema/schema.graphql`

Remplacer la signature :

```graphql
# Avant
deprovisionModule(moduleName: String!): Boolean!

# Apres
type DeprovisionResult {
  success: Boolean!
  postgresCleared: Boolean!
  storageCleared: Boolean!
  storageSkippedReason: String
  korsDataCleared: Boolean!
  error: MutationError
}

# Dans Mutation :
deprovisionModule(moduleName: String!, forceDeleteStorage: Boolean): DeprovisionResult!
```

**Fichier 2 :** `kors-api/internal/domain/provisioning/provisioning.go`

Ajouter `DeprovisionReport` :

```go
type DeprovisionReport struct {
    Success              bool
    PostgresCleared      bool
    StorageCleared       bool
    StorageSkippedReason string
    KorsDataCleared      bool
}
```

Modifier `StorageProvisioner` pour accepter `force bool` :

```go
type StorageProvisioner interface {
    ProvisionBucket(ctx context.Context, moduleName string) error
    DeprovisionBucket(ctx context.Context, moduleName string, force bool) error
}
```

**Fichier 3 :** `kors-api/internal/domain/identity/identity.go`

Ajouter `Delete` dans l'interface `Repository` :

```go
type Repository interface {
    Create(ctx context.Context, id *Identity) error
    GetByID(ctx context.Context, id uuid.UUID) (*Identity, error)
    GetByExternalID(ctx context.Context, externalID string) (*Identity, error)
    Delete(ctx context.Context, id uuid.UUID) error // NOUVEAU
}
```

**Fichier 4 :** `kors-api/internal/adapter/postgres/identity_repo.go`

Implementer `Delete` :

```go
func (r *IdentityRepository) Delete(ctx context.Context, id uuid.UUID) error {
    _, err := r.Pool.Exec(ctx, "DELETE FROM kors.identities WHERE id = $1", id)
    return err
}
```

**Fichier 5 :** `kors-api/internal/domain/permission/permission.go`

Ajouter `DeleteForIdentity` dans l'interface `Repository` :

```go
type Repository interface {
    Create(ctx context.Context, p *Permission) error
    Delete(ctx context.Context, id uuid.UUID) error
    FindForIdentity(ctx context.Context, identityID uuid.UUID) ([]*Permission, error)
    Check(ctx context.Context, identityID uuid.UUID, action string, resourceID *uuid.UUID, resourceTypeID *uuid.UUID) (bool, error)
    DeleteForIdentity(ctx context.Context, identityID uuid.UUID) error // NOUVEAU
}
```

**Fichier 6 :** `kors-api/internal/adapter/postgres/permission_repo.go`

Implementer `DeleteForIdentity` :

```go
func (r *PermissionRepository) DeleteForIdentity(ctx context.Context, identityID uuid.UUID) error {
    _, err := r.Pool.Exec(ctx, "DELETE FROM kors.permissions WHERE identity_id = $1", identityID)
    return err
}
```

**Fichier 7 :** `kors-api/internal/adapter/minio/provisioner.go`

Modifier `DeprovisionBucket` pour vider le bucket si `force=true` :

```go
func (p *MinioProvisioner) DeprovisionBucket(ctx context.Context, moduleName string, force bool) error {
    cleanName := strings.ReplaceAll(strings.ToLower(moduleName), "_", "-")
    bucketName := fmt.Sprintf("module-%s", cleanName)

    exists, err := p.Client.BucketExists(ctx, bucketName)
    if err != nil || !exists { return nil }

    if force {
        objectsCh := p.Client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})
        for object := range objectsCh {
            if object.Err != nil { continue }
            _ = p.Client.RemoveObject(ctx, bucketName, object.Key, minio.RemoveObjectOptions{})
        }
    }

    if err := p.Client.RemoveBucket(ctx, bucketName); err != nil {
        return fmt.Errorf("bucket %s not deleted (may not be empty): %w", bucketName, err)
    }
    return nil
}
```

**Fichier 8 :** `kors-api/internal/usecase/provision_module.go`

Reecrire `Deprovision` :

```go
func (uc *ModuleGovernanceUseCase) Deprovision(ctx context.Context, moduleName string, identityID uuid.UUID, forceDeleteStorage bool) (*provisioning.DeprovisionReport, error) {
    if err := validateModuleName(moduleName); err != nil { return nil, err }
    if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }

    report := &provisioning.DeprovisionReport{}

    // 1. Recuperer l'identite KORS du module
    ident, _ := uc.IdentityRepo.GetByExternalID(ctx, moduleName)

    // 2. Supprimer les permissions KORS du module
    if ident != nil {
        if err := uc.PermissionRepo.DeleteForIdentity(ctx, ident.ID); err == nil {
            report.KorsDataCleared = true
        }
        // 3. Supprimer l'identite KORS
        _ = uc.IdentityRepo.Delete(ctx, ident.ID)
    } else {
        report.KorsDataCleared = true // rien a nettoyer
    }

    // 4. Deprovisionner MinIO
    if uc.StorageProvisioner != nil {
        err := uc.StorageProvisioner.DeprovisionBucket(ctx, moduleName, forceDeleteStorage)
        if err != nil {
            report.StorageSkippedReason = err.Error()
        } else {
            report.StorageCleared = true
        }
    }

    // 5. Deprovisionner Postgres
    if err := uc.Provisioner.DeprovisionModule(ctx, moduleName); err != nil {
        return report, fmt.Errorf("failed to deprovision postgres: %w", err)
    }
    report.PostgresCleared = true

    // 6. Supprimer du registre
    _ = uc.Provisioner.UnregisterModule(ctx, moduleName)

    report.Success = true
    return report, nil
}
```

**Fichier 9 :** `kors-api/internal/graph/resolvers/schema.resolvers.go`

Mettre a jour le resolver `DeprovisionModule` pour accepter `forceDeleteStorage *bool` et
retourner `*model.DeprovisionResult`. Passer `false` si le parametre est nil.

Regenerer gqlgen apres modification du schema.

---

### TACHE 2.4 — Mutation rotateModuleCredentials

**Probleme identifie :** Les credentials Postgres generes a la provision ne peuvent jamais
etre changes. En cas de compromission, il n'y a aucun moyen de rotation.

**Fichier 1 :** `kors-api/internal/graph/schema/schema.graphql`

```graphql
type Mutation {
  # ... mutations existantes ...
  """
  Genere un nouveau mot de passe Postgres pour le role du module.
  Retourne les nouveaux credentials complets. Admin requis.
  """
  rotateModuleCredentials(moduleName: String!): ProvisioningResult!
}
```

**Fichier 2 :** `kors-api/internal/adapter/postgres/provisioner.go`

Implementer `RotatePassword` (deja declare dans l'interface a la tache 2.1) :

```go
func (p *PostgresProvisioner) RotatePassword(ctx context.Context, moduleName string) (*provisioning.ModuleCredentials, error) {
    if err := validateModuleName(moduleName); err != nil { return nil, err }
    username := fmt.Sprintf("user_%s", moduleName)
    newPassword := generateRandomPassword(16)
    _, err := p.Pool.Exec(ctx, fmt.Sprintf("ALTER ROLE %s WITH PASSWORD '%s'", username, newPassword))
    if err != nil { return nil, fmt.Errorf("failed to rotate password: %w", err) }
    bucketName := fmt.Sprintf("module-%s", strings.ReplaceAll(strings.ToLower(moduleName), "_", "-"))
    return &provisioning.ModuleCredentials{
        ModuleName:       moduleName,
        Schema:           moduleName,
        Username:         username,
        Password:         newPassword,
        ConnectionString: buildConnectionString(username, newPassword, moduleName),
        BucketName:       bucketName,
    }, nil
}
```

**Fichier 3 :** `kors-api/internal/usecase/provision_module.go`

Ajouter une methode `Rotate` :

```go
func (uc *ModuleGovernanceUseCase) Rotate(ctx context.Context, moduleName string, identityID uuid.UUID) (*provisioning.ModuleCredentials, error) {
    if err := validateModuleName(moduleName); err != nil { return nil, err }
    if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }
    return uc.Provisioner.RotatePassword(ctx, moduleName)
}
```

Implementer le resolver dans `schema.resolvers.go`. Regenerer gqlgen.

---

## SPRINT 3 — Coherence interne et transactions

---

### TACHE 3.1 — Supprimer le type assert dans CreateResource

**Probleme identifie :** `create_resource.go` fait des type asserts vers
`*postgres.ResourceRepository` et `*postgres.EventRepository`, cassant l'architecture
hexagonale. Les use cases ne doivent pas connaitre les adapters. Les tests unitaires
avec mocks deviennent impossibles car le mock n'est jamais un type Postgres concret.

**Fichier 1 :** `kors-api/internal/domain/resource/resource.go`

Ajouter `CreateWithTx` dans l'interface (importer `pgx`) :

```go
import "github.com/jackc/pgx/v5"

type Repository interface {
    Create(ctx context.Context, res *Resource) error
    CreateWithTx(ctx context.Context, tx pgx.Tx, res *Resource) error // NOUVEAU
    GetByID(ctx context.Context, id uuid.UUID) (*Resource, error)
    Update(ctx context.Context, res *Resource) error
    UpdateWithTx(ctx context.Context, tx pgx.Tx, res *Resource) error // NOUVEAU (pour tache 3.2)
    List(ctx context.Context, first int, after *uuid.UUID, typeName *string) ([]*Resource, bool, int, error)
    SoftDelete(ctx context.Context, id uuid.UUID) error
}
```

**Fichier 2 :** `kors-api/internal/domain/event/event.go`

Ajouter `CreateWithTx` dans l'interface :

```go
import "github.com/jackc/pgx/v5"

type Repository interface {
    Create(ctx context.Context, e *Event) error
    CreateWithTx(ctx context.Context, tx pgx.Tx, e *Event) error // NOUVEAU
}
```

**Fichier 3 :** `kors-api/internal/adapter/postgres/resource_repo.go`

Implementer `UpdateWithTx` (pattern identique a `CreateWithTx` deja present) :

```go
func (r *ResourceRepository) UpdateWithTx(ctx context.Context, tx pgx.Tx, res *resource.Resource) error {
    _, err := tx.Exec(ctx, `
        UPDATE kors.resources SET state = $1, metadata = $2, updated_at = $3
        WHERE id = $4 AND deleted_at IS NULL
    `, res.State, res.Metadata, res.UpdatedAt, res.ID)
    return err
}
```

**Fichier 4 :** `kors-api/internal/usecase/create_resource.go`

Supprimer l'import `postgres` et remplacer les deux type asserts par des appels d'interface :

```go
// Supprimer cet import :
// "github.com/haksolot/kors/kors-api/internal/adapter/postgres"

// Remplacer le bloc entier avec type asserts par :
tx, err := uc.Pool.Begin(ctx)
if err != nil { return nil, fmt.Errorf("failed to begin transaction: %w", err) }
defer tx.Rollback(ctx)

if err := uc.ResourceRepo.CreateWithTx(ctx, tx, res); err != nil {
    return nil, fmt.Errorf("failed to persist resource: %w", err)
}
if err := uc.EventRepo.CreateWithTx(ctx, tx, ev); err != nil {
    return nil, fmt.Errorf("failed to persist event: %w", err)
}

if uc.EventPublisher != nil {
    if err := uc.EventPublisher.Publish(ctx, ev); err != nil {
        return nil, fmt.Errorf("bus error: %w", err)
    }
}

if err := tx.Commit(ctx); err != nil {
    return nil, fmt.Errorf("failed to commit transaction: %w", err)
}
```

**Fichier 5 :** `kors-api/internal/usecase/mocks/mocks.go`

Ajouter les nouvelles methodes dans les mocks `ResourceRepo` et `EventRepo` :

```go
func (m *ResourceRepo) CreateWithTx(_ context.Context, _ pgx.Tx, res *resource.Resource) error {
    return m.Create(context.Background(), res)
}
func (m *ResourceRepo) UpdateWithTx(_ context.Context, _ pgx.Tx, res *resource.Resource) error {
    return m.Update(context.Background(), res)
}
func (m *EventRepo) CreateWithTx(_ context.Context, _ pgx.Tx, e *event.Event) error {
    return m.Create(context.Background(), e)
}
```

---

### TACHE 3.2 — Transaction atomique dans TransitionResource

**Probleme identifie :** `transition_resource.go` met a jour la resource et cree l'evenement
en deux operations separees sans transaction. Si `EventRepo.Create` echoue, la resource est
dans le nouvel etat sans aucune trace d'audit. De plus, l'echec de l'evenement est loggue
en `Warn` mais l'operation retourne quand meme `success` — inconsistance silencieuse.

**Fichier :** `kors-api/internal/usecase/transition_resource.go`

Ajouter `Pool *pgxpool.Pool` dans la struct :

```go
type TransitionResourceUseCase struct {
    Pool             *pgxpool.Pool  // NOUVEAU
    ResourceRepo     resource.Repository
    ResourceTypeRepo resourcetype.Repository
    EventRepo        event.Repository
    PermissionRepo   permission.Repository
    EventPublisher   event.Publisher
    Logger           zerolog.Logger
}
```

Dans `Execute`, apres la validation de la transition, remplacer les appels `Update` et
`EventRepo.Create` par une transaction atomique :

```go
// Remplacer le bloc "5. Update Resource" et "7. Persist Event" par :
tx, err := uc.Pool.Begin(ctx)
if err != nil { return nil, fmt.Errorf("failed to begin transaction: %w", err) }
defer tx.Rollback(ctx)

if err := uc.ResourceRepo.UpdateWithTx(ctx, tx, res); err != nil {
    return nil, fmt.Errorf("failed to update resource: %w", err)
}
if err := uc.EventRepo.CreateWithTx(ctx, tx, ev); err != nil {
    return nil, fmt.Errorf("failed to persist transition event: %w", err)
}

if uc.EventPublisher != nil {
    if err := uc.EventPublisher.Publish(ctx, ev); err != nil {
        // NATS n'est pas transactionnel. On logue l'echec mais on commite quand meme
        // pour ne pas perdre la transition. Un worker de retry peut recouvrer.
        uc.Logger.Warn().Err(err).Msg("failed to broadcast transition event on NATS")
    }
}

if err := tx.Commit(ctx); err != nil {
    return nil, fmt.Errorf("failed to commit transition: %w", err)
}
```

Mettre a jour le wiring dans `resolver.go` pour passer `pool` au `TransitionResourceUseCase`.

---

### TACHE 3.3 — Proteger grantPermission par une verification du caller

**Probleme identifie :** `GrantPermissionUseCase.Execute` n'a aucune garde sur qui appelle.
N'importe quelle identite authentifiee peut accorder des permissions `admin` globales
a n'importe qui. Le resolver ne passe pas non plus le CallerID.

**Fichier 1 :** `kors-api/internal/usecase/grant_permission.go`

Ajouter `CallerID` dans l'input et les checks :

```go
type GrantPermissionInput struct {
    CallerID       uuid.UUID  // NOUVEAU
    IdentityID     uuid.UUID
    ResourceID     *uuid.UUID
    ResourceTypeID *uuid.UUID
    Action         string
    ExpiresAt      *time.Time
}

type GrantPermissionUseCase struct {
    Repo           permission.Repository
    PermissionRepo permission.Repository // pour checker les droits du caller
}

func (uc *GrantPermissionUseCase) Execute(ctx context.Context, input GrantPermissionInput) (*permission.Permission, error) {
    if !validActions[input.Action] {
        return nil, fmt.Errorf("invalid action %q: must be one of read, write, transition, admin", input.Action)
    }

    if input.Action == "admin" {
        // Seul un admin global peut accorder admin
        allowed, err := uc.PermissionRepo.Check(ctx, input.CallerID, "admin", nil, nil)
        if err != nil { return nil, fmt.Errorf("failed to check caller permission: %w", err) }
        if !allowed { return nil, fmt.Errorf("only global admins can grant admin permissions") }
    } else {
        // write ou admin suffisent pour deleguer les autres permissions
        isAdmin, _ := uc.PermissionRepo.Check(ctx, input.CallerID, "admin", nil, nil)
        hasWrite, _ := uc.PermissionRepo.Check(ctx, input.CallerID, "write", nil, nil)
        if !isAdmin && !hasWrite {
            return nil, fmt.Errorf("write or admin permission required to grant permissions")
        }
    }

    p := &permission.Permission{
        ID: uuid.New(), IdentityID: input.IdentityID,
        ResourceID: input.ResourceID, ResourceTypeID: input.ResourceTypeID,
        Action: input.Action, ExpiresAt: input.ExpiresAt, CreatedAt: time.Now(),
    }
    return p, uc.Repo.Create(ctx, p)
}
```

**Fichier 2 :** `kors-api/internal/graph/resolvers/schema.resolvers.go`

Passer `CallerID: getIdentityID(ctx)` dans `GrantPermissionInput`.

**Fichier 3 :** `kors-api/internal/graph/resolvers/resolver.go`

Mettre a jour l'instanciation de `GrantPermissionUseCase` :

```go
GrantPermissionUseCase: &usecase.GrantPermissionUseCase{Repo: pRepo, PermissionRepo: pRepo},
```

---

## SPRINT 4 — Requetabilite GraphQL complete

---

### TACHE 4.1 — Query events avec pagination et filtres

**Probleme identifie :** Il n'existe aucune query HTTP pour lire l'historique des evenements.
Un module qui veut afficher un audit trail doit requeter directement Postgres, rompant le
contrat d'isolation.

**Fichier 1 :** `kors-api/internal/domain/event/event.go`

Ajouter une struct de filtre et etendre l'interface `Repository` :

```go
type ListFilter struct {
    ResourceID *uuid.UUID
    IdentityID *uuid.UUID
    Type       *string
}

type Repository interface {
    Create(ctx context.Context, e *Event) error
    CreateWithTx(ctx context.Context, tx pgx.Tx, e *Event) error
    List(ctx context.Context, filter ListFilter, first int, after *uuid.UUID) ([]*Event, bool, int, error) // NOUVEAU
    GetByID(ctx context.Context, id uuid.UUID) (*Event, error)                                            // NOUVEAU (pour entity resolver)
}
```

**Fichier 2 :** `kors-api/internal/adapter/postgres/event_repo.go`

Implementer `List` et `GetByID` avec le meme pattern de construction dynamique que
`resource_repo.go`. Utiliser `ORDER BY created_at DESC`, pagination cursor sur `created_at`.

**Fichier 3 :** `kors-api/internal/usecase/list_events.go` (nouveau fichier)

```go
package usecase

import (
    "context"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/event"
    "github.com/haksolot/kors/shared/pagination"
)

type ListEventsInput struct {
    ResourceID *uuid.UUID
    IdentityID *uuid.UUID
    Type       *string
    First      int
    After      *string
}

type ListEventsResult struct {
    Events      []*event.Event
    HasNextPage bool
    TotalCount  int
}

type ListEventsUseCase struct {
    Repo event.Repository
}

func (uc *ListEventsUseCase) Execute(ctx context.Context, input ListEventsInput) (*ListEventsResult, error) {
    if input.First == 0 { input.First = 20 }
    var after *uuid.UUID
    if input.After != nil {
        raw, err := pagination.DecodeCursor(*input.After)
        if err != nil { return nil, fmt.Errorf("invalid cursor: %w", err) }
        id, err := uuid.Parse(raw)
        if err != nil { return nil, fmt.Errorf("invalid cursor id: %w", err) }
        after = &id
    }
    filter := event.ListFilter{ResourceID: input.ResourceID, IdentityID: input.IdentityID, Type: input.Type}
    events, hasNext, total, err := uc.Repo.List(ctx, filter, input.First, after)
    if err != nil { return nil, err }
    return &ListEventsResult{Events: events, HasNextPage: hasNext, TotalCount: total}, nil
}
```

**Fichier 4 :** `kors-api/internal/graph/schema/schema.graphql`

```graphql
type Query {
  # ... queries existantes ...
  """
  Liste les evenements avec filtres optionnels et pagination.
  """
  events(
    resourceId: UUID
    identityId: UUID
    type: String
    first: Int
    after: String
  ): EventConnection!
}
```

Implementer le resolver, ajouter `ListEventsUseCase` dans `Resolver` et `NewResolver`.
Regenerer gqlgen.

---

### TACHE 4.2 — Query permissions

**Probleme identifie :** Un module ne peut pas savoir quelles permissions il a via GraphQL.
Il ne peut pas non plus verifier les droits d'un utilisateur. Pas de query `permissions`.

**Fichier 1 :** `kors-api/internal/domain/permission/permission.go`

Etendre l'interface `Repository` :

```go
type Repository interface {
    Create(ctx context.Context, p *Permission) error
    Delete(ctx context.Context, id uuid.UUID) error
    DeleteForIdentity(ctx context.Context, identityID uuid.UUID) error
    FindForIdentity(ctx context.Context, identityID uuid.UUID) ([]*Permission, error)
    Check(ctx context.Context, identityID uuid.UUID, action string, resourceID *uuid.UUID, resourceTypeID *uuid.UUID) (bool, error)
    List(ctx context.Context, identityID *uuid.UUID, resourceID *uuid.UUID, resourceTypeID *uuid.UUID) ([]*Permission, error) // NOUVEAU
    GetByID(ctx context.Context, id uuid.UUID) (*Permission, error) // NOUVEAU (pour entity resolver)
}
```

**Fichier 2 :** `kors-api/internal/adapter/postgres/permission_repo.go`

Implementer `List` avec WHERE dynamique et filtrage `WHERE expires_at IS NULL OR expires_at > NOW()`.
Implementer `GetByID`.

**Fichier 3 :** `kors-api/internal/usecase/list_permissions.go` (nouveau fichier)

Use case simple qui verifie que le caller a `admin` ou interroge ses propres permissions,
puis appelle `PermissionRepo.List`.

**Fichier 4 :** `kors-api/internal/graph/schema/schema.graphql`

```graphql
type Query {
  # ... queries existantes ...
  """
  Liste les permissions avec filtres optionnels. Admin requis ou self-query.
  """
  permissions(identityId: UUID, resourceId: UUID, resourceTypeId: UUID): [Permission!]!
}
```

Implementer le resolver, ajouter dans `Resolver` et `NewResolver`. Regenerer gqlgen.

---

### TACHE 4.3 — Queries identities

**Probleme identifie :** Aucune query pour lire les identites. Un module ne peut pas lister
les utilisateurs qu'il a enregistres.

**Fichier 1 :** `kors-api/internal/domain/identity/identity.go`

Etendre l'interface `Repository` :

```go
type Repository interface {
    Create(ctx context.Context, id *Identity) error
    GetByID(ctx context.Context, id uuid.UUID) (*Identity, error)
    GetByExternalID(ctx context.Context, externalID string) (*Identity, error)
    Delete(ctx context.Context, id uuid.UUID) error
    List(ctx context.Context, identityType *string, first int, after *uuid.UUID) ([]*Identity, bool, int, error) // NOUVEAU
}
```

**Fichier 2 :** `kors-api/internal/adapter/postgres/identity_repo.go`

Implementer `List` avec filtre `type` optionnel, `ORDER BY created_at DESC`, pagination cursor.

**Fichier 3 :** `kors-api/internal/graph/schema/schema.graphql`

```graphql
type IdentityConnection {
  edges: [IdentityEdge!]!
  pageInfo: PageInfo!
  totalCount: Int!
}
type IdentityEdge {
  cursor: String!
  node: Identity!
}

type Query {
  # ... queries existantes ...
  """
  Retourne une identite par son UUID interne.
  """
  identity(id: UUID!): Identity

  """
  Liste les identites avec filtre optionnel sur le type. Admin requis.
  """
  identities(type: String, first: Int, after: String): IdentityConnection!
}
```

Creer `get_identity.go` et `list_identities.go` use cases (verifier `admin` pour `identities`,
librement accessible pour `identity` si l'appelant connait l'UUID).
Ajouter dans `Resolver` et `NewResolver`. Regenerer gqlgen.

---

### TACHE 4.4 — Filtres avances sur la query resources

**Probleme identifie :** `resources` filtre uniquement par `typeName`. Un module voudra
filtrer par `state`, par date de creation, ce qui lui evite de tout charger cote client.

**Fichier 1 :** `kors-api/internal/domain/resource/resource.go`

Ajouter une struct de filtre et modifier la signature de `List` :

```go
type ListFilter struct {
    TypeName      *string
    State         *string
    CreatedAfter  *time.Time
    CreatedBefore *time.Time
}

type Repository interface {
    Create(ctx context.Context, res *Resource) error
    CreateWithTx(ctx context.Context, tx pgx.Tx, res *Resource) error
    GetByID(ctx context.Context, id uuid.UUID) (*Resource, error)
    Update(ctx context.Context, res *Resource) error
    UpdateWithTx(ctx context.Context, tx pgx.Tx, res *Resource) error
    List(ctx context.Context, first int, after *uuid.UUID, filter ListFilter) ([]*Resource, bool, int, error) // signature modifiee
    SoftDelete(ctx context.Context, id uuid.UUID) error
}
```

**Fichier 2 :** `kors-api/internal/adapter/postgres/resource_repo.go`

Adapter `List` pour construire la requete dynamiquement avec le `ListFilter`.

**Fichier 3 :** Mettre a jour partout ou `List` est appele :

- `kors-api/internal/usecase/list_resources.go` : ajouter `State *string`, `CreatedAfter *time.Time`, `CreatedBefore *time.Time` dans `ListResourcesInput` et passer `resource.ListFilter`.
- `kors-api/internal/graph/schema/schema.graphql` : ajouter les parametres dans `resources(...)`.
- `kors-api/internal/graph/resolvers/schema.resolvers.go` : passer les nouveaux params.
- Mocks dans `mocks.go` : mettre a jour la signature.

Regenerer gqlgen.

---

### TACHE 4.5 — Implementer les entity resolvers federatifs manquants

**Probleme identifie :** Cinq resolvers dans `entity.resolvers.go` font `panic`. La
federation Apollo ne peut pas resoudre ces entites depuis les subgraphs des modules.

**Fichier :** `kors-api/internal/graph/resolvers/entity.resolvers.go`

Implementer chaque resolver avec les repos disponibles dans `Resolver` :

```go
// FindEventByID : utiliser ListEventsUseCase.Repo.GetByID (apres tache 4.1)
func (r *entityResolver) FindEventByID(ctx context.Context, id uuid.UUID) (*model.Event, error) {
    e, err := r.ListEventsUseCase.Repo.GetByID(ctx, id)
    if err != nil || e == nil { return nil, err }
    return &model.Event{ID: e.ID, Type: e.Type, Payload: e.Payload, CreatedAt: e.CreatedAt}, nil
}

// FindIdentityByID : utiliser IdentityRepo directement (accessible via wiring)
func (r *entityResolver) FindIdentityByID(ctx context.Context, id uuid.UUID) (*model.Identity, error) {
    // ajouter IdentityRepo dans Resolver si pas encore present
    ident, err := r.IdentityRepo.GetByID(ctx, id)
    if err != nil || ident == nil { return nil, err }
    return &model.Identity{ID: ident.ID, Name: ident.Name, Type: ident.Type, CreatedAt: ident.CreatedAt, UpdatedAt: ident.UpdatedAt}, nil
}

// FindPermissionByID : utiliser PermissionRepo.GetByID (apres tache 4.2)
func (r *entityResolver) FindPermissionByID(ctx context.Context, id uuid.UUID) (*model.Permission, error) {
    p, err := r.ListPermissionsUseCase.Repo.GetByID(ctx, id)
    if err != nil || p == nil { return nil, err }
    return &model.Permission{ID: p.ID, Action: p.Action, ExpiresAt: p.ExpiresAt, CreatedAt: p.CreatedAt}, nil
}

// FindResourceTypeByID : utiliser GetResourceTypeUseCase.Repo.GetByID
func (r *entityResolver) FindResourceTypeByID(ctx context.Context, id uuid.UUID) (*model.ResourceType, error) {
    rt, err := r.GetResourceTypeUseCase.Repo.GetByID(ctx, id)
    if err != nil || rt == nil { return nil, err }
    desc := rt.Description
    return &model.ResourceType{ID: rt.ID, Name: rt.Name, Description: &desc, JSONSchema: rt.JSONSchema, Transitions: rt.Transitions, CreatedAt: rt.CreatedAt, UpdatedAt: rt.UpdatedAt}, nil
}

// FindRevisionByID : utiliser CreateRevisionUseCase.RevisionRepo.GetByID
func (r *entityResolver) FindRevisionByID(ctx context.Context, id uuid.UUID) (*model.Revision, error) {
    rev, err := r.CreateRevisionUseCase.RevisionRepo.GetByID(ctx, id)
    if err != nil || rev == nil { return nil, err }
    return &model.Revision{ID: rev.ID, Snapshot: rev.Snapshot, FilePath: rev.FilePath, CreatedAt: rev.CreatedAt}, nil
}
```

Si `IdentityRepo` n'est pas encore dans la struct `Resolver`, l'ajouter au wiring dans
`resolver.go` et `NewResolver`.

---

## SPRINT 5 — SDK et exemple de module

---

### TACHE 5.1 — Completer le SDK Go avec toutes les operations

**Probleme identifie :** Le SDK Go ne couvre que 6 des 14+ operations disponibles dans le
schema. Un module ne peut faire ni `grantPermission`, ni `createRevision`, ni `createIdentity`,
ni aucune operation d'administration via le SDK.

**Fichier :** `sdk/go/operations.graphql`

Ajouter toutes les operations manquantes :

```graphql
mutation CreateIdentity($input: CreateIdentityInput!) {
  createIdentity(input: $input) {
    success
    identity { id externalId name type createdAt }
    error { code message }
  }
}

mutation GrantPermission($input: GrantPermissionInput!) {
  grantPermission(input: $input) {
    success
    permission { id action expiresAt createdAt }
    error { code message }
  }
}

mutation CreateRevision($input: CreateRevisionInput!) {
  createRevision(input: $input) {
    success
    revision { id filePath createdAt }
    error { code message }
  }
}

mutation DeleteResource($id: UUID!) {
  deleteResource(id: $id) {
    success
    error { code message }
  }
}

mutation ProvisionModule($moduleName: String!) {
  provisionModule(moduleName: $moduleName) {
    success
    moduleName
    schema
    username
    password
    connectionString
    bucketName
    error { code message }
  }
}

mutation DeprovisionModule($moduleName: String!, $forceDeleteStorage: Boolean) {
  deprovisionModule(moduleName: $moduleName, forceDeleteStorage: $forceDeleteStorage) {
    success
    postgresCleared
    storageCleared
    korsDataCleared
    storageSkippedReason
    error { code message }
  }
}

mutation RotateModuleCredentials($moduleName: String!) {
  rotateModuleCredentials(moduleName: $moduleName) {
    success
    moduleName
    username
    password
    connectionString
    error { code message }
  }
}

query GetResourceType($name: String!) {
  resourceType(name: $name) {
    id name description jsonSchema transitions createdAt updatedAt
  }
}

query ListResourceTypes {
  resourceTypes { id name description createdAt }
}

query ListEvents($resourceId: UUID, $identityId: UUID, $type: String, $first: Int, $after: String) {
  events(resourceId: $resourceId, identityId: $identityId, type: $type, first: $first, after: $after) {
    totalCount
    edges {
      cursor
      node { id type payload createdAt }
    }
    pageInfo { hasNextPage endCursor }
  }
}

query ListPermissions($identityId: UUID, $resourceId: UUID, $resourceTypeId: UUID) {
  permissions(identityId: $identityId, resourceId: $resourceId, resourceTypeId: $resourceTypeId) {
    id action expiresAt createdAt
  }
}

query GetIdentity($id: UUID!) {
  identity(id: $id) { id externalId name type createdAt }
}

query ListIdentities($type: String, $first: Int, $after: String) {
  identities(type: $type, first: $first, after: $after) {
    totalCount
    edges { cursor node { id externalId name type } }
    pageInfo { hasNextPage endCursor }
  }
}

query GetModule($name: String!) {
  module(name: $name) { name schemaName pgUsername bucketName provisionedAt }
}

query ListModulesDetailed {
  provisionedModules { name schemaName pgUsername bucketName provisionedAt }
}
```

Regenerer le SDK Go depuis `sdk/go/` apres que le schema kors-api est stable :

```
cd sdk/go && go run github.com/Khan/genqlient genqlient.yaml
```

---

### TACHE 5.2 — Completer l'exemple TMS

**Probleme identifie 1 :** `tms.resolvers.go` contient `panic(fmt.Errorf("not implemented..."))`.
L'exemple de module de reference panique en production.

**Probleme identifie 2 :** `main.go` utilise la meme `DATABASE_URL` pour les migrations
(credentials admin) et la connexion applicative. Le module devrait tourner avec son propre
role restreint `user_tms`.

**Fichier 1 :** `examples/module-example/internal/store/tool_store.go`

Ajouter `ListAll` :

```go
func (s *ToolStore) ListAll(ctx context.Context) ([]*model.Tool, error) {
    // Grace au search_path configure (tache 1.5), "tools" est accessible directement
    rows, err := s.Pool.Query(ctx, "SELECT id, serial_number, model, diameter, length FROM tools")
    if err != nil { return nil, err }
    defer rows.Close()
    var tools []*model.Tool
    for rows.Next() {
        var t model.Tool
        if err := rows.Scan(&t.ID, &t.SerialNumber, &t.Model, &t.Diameter, &t.Length); err != nil {
            return nil, err
        }
        tools = append(tools, &t)
    }
    return tools, nil
}
```

**Fichier 2 :** `examples/module-example/internal/graph/resolvers/tms.resolvers.go`

Remplacer le panic par une implementation reelle :

```go
func (r *queryResolver) Tools(ctx context.Context) ([]*model.Resource, error) {
    tools, err := r.Store.ListAll(ctx)
    if err != nil { return nil, err }
    result := make([]*model.Resource, 0, len(tools))
    for _, t := range tools {
        result = append(result, &model.Resource{Id: t.ID.String()})
    }
    return result, nil
}
```

**Fichier 3 :** `examples/module-example/cmd/server/main.go`

Separer la connexion admin (migrations) de la connexion applicative (runtime) :

```go
// 1. Connexion admin pour les migrations uniquement
adminDBURL := os.Getenv("DATABASE_URL")
adminDB := stdlib.OpenDB(...)
_ = goose.Up(adminDB, "/migrations")
adminDB.Close()

// 2. Connexion applicative avec les credentials du module
// MODULE_DATABASE_URL = connectionString retournee par provisionModule
moduleDBURL := getEnv("MODULE_DATABASE_URL", adminDBURL) // fallback en dev
pool, _ := pgxpool.New(context.Background(), moduleDBURL)
```

Documenter dans le README de l'exemple que `MODULE_DATABASE_URL` doit etre la
`connectionString` retournee par `provisionModule`, pas la DATABASE_URL admin.

---

## TACHES TRANSVERSES — A appliquer en continu

---

### TACHE T.1 — Maintenir les mocks synchronises avec les interfaces

Apres chaque modification d'une interface de Repository, mettre a jour
`kors-api/internal/usecase/mocks/mocks.go`. Le compilateur Go listera les interfaces
non satisfaites — ne pas lancer `go build` en ignorant les erreurs.

Nouvelles methodes a ajouter au fil des sprints :

- `ResourceRepo` : `CreateWithTx`, `UpdateWithTx`
- `EventRepo` : `CreateWithTx`, `GetByID`, `List`
- `PermissionRepo` : `DeleteForIdentity`, `List`, `GetByID`
- `IdentityRepo` : `Delete`, `List`

---

### TACHE T.2 — Maintenir le wiring dans resolver.go a jour

Apres chaque nouveau use case, mettre a jour dans l'ordre :

1. La struct `Resolver` dans `resolver.go` (ajouter le champ)
2. `NewResolver` dans `resolver.go` (instancier et injecter les dependances)
3. Le resolver dans `schema.resolvers.go` (appeler le use case)

---

### TACHE T.3 — Repercuter les changements de types dans shared/schema/kors.graphql

`shared/schema/kors.graphql` est le contrat partage pour la federation. Il doit rester
synchronise avec le schema principal. Apres chaque tache qui modifie des types de base
(Identity, Resource, Event, Permission, Revision, ResourceType et leurs connections),
repercuter les changements dans `shared/schema/kors.graphql`. Ne pas y mettre les mutations
et queries specifiques a kors-api.

---

## Ordre d'execution recommande

```
Sprint 1  :  1.1 -> 1.2 -> 1.3 -> 1.4 -> 1.5
Sprint 2  :  2.1 -> 2.2 -> 2.3 -> 2.4
Sprint 3  :  3.1 -> 3.2 -> 3.3
Sprint 4  :  4.1 -> 4.2 -> 4.3 -> 4.4 -> 4.5
Sprint 5  :  5.1 (apres que tous les schemas sont stables) -> 5.2
Transvers :  T.1 et T.2 apres chaque tache qui modifie une interface ou ajoute un use case
             T.3 apres chaque tache qui modifie un type de base dans schema.graphql
```

Verification apres chaque sprint :

```
cd kors-api && go build ./... && go test ./internal/domain/... ./internal/usecase/...
```
