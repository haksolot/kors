package provisioning

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ModuleCredentials represents the database access info for a provisioned module.
type ModuleCredentials struct {
	ModuleName       string
	Schema           string
	Username         string
	Password         string
	ConnectionString string // ex: postgres://user_tms:xxx@host/db?options=-csearch_path%3Dtms%2Ckors
	BucketName       string // ex: module-tms
}

type ModuleRecord struct {
	Name          string
	SchemaName    string
	PgUsername    string
	MinioBucket   string
	IdentityID    *uuid.UUID
	ProvisionedAt time.Time
	ProvisionedBy *uuid.UUID
}

type DeprovisionReport struct {
	Success              bool
	PostgresCleared      bool
	StorageCleared       bool
	StorageSkippedReason string
	KorsDataCleared      bool
}

// Service defines the contract for provisioning new modules (typically DB).
type Service interface {
	ProvisionModule(ctx context.Context, moduleName string) (*ModuleCredentials, error)
	DeprovisionModule(ctx context.Context, moduleName string) error
	ListModules(ctx context.Context) ([]string, error)
	RegisterModule(ctx context.Context, record *ModuleRecord) error                        // NOUVEAU
	UnregisterModule(ctx context.Context, moduleName string) error                         // NOUVEAU
	GetModule(ctx context.Context, moduleName string) (*ModuleRecord, error)               // NOUVEAU
	RotatePassword(ctx context.Context, moduleName string) (*ModuleCredentials, error)     // NOUVEAU (tache 2.4)
}

// StorageProvisioner defines the contract for provisioning module storage (MinIO).
type StorageProvisioner interface {
	ProvisionBucket(ctx context.Context, moduleName string) error
	DeprovisionBucket(ctx context.Context, moduleName string, force bool) error
}

