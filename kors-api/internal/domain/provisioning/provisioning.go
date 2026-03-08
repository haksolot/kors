package provisioning

import (
	"context"
)

// ModuleCredentials represents the database access info for a provisioned module.
type ModuleCredentials struct {
	ModuleName string
	Schema     string
	Username   string
	Password   string
}

// Service defines the contract for provisioning new modules (typically DB).
type Service interface {
	ProvisionModule(ctx context.Context, moduleName string) (*ModuleCredentials, error)
	DeprovisionModule(ctx context.Context, moduleName string) error
	ListModules(ctx context.Context) ([]string, error)
}

// StorageProvisioner defines the contract for provisioning module storage (MinIO).
type StorageProvisioner interface {
	ProvisionBucket(ctx context.Context, moduleName string) error
	DeprovisionBucket(ctx context.Context, moduleName string) error
}

