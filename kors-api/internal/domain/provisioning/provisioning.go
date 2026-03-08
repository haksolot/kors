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

// Service defines the contract for provisioning new modules.
type Service interface {
	ProvisionModule(ctx context.Context, moduleName string) (*ModuleCredentials, error)
	DeprovisionModule(ctx context.Context, moduleName string) error
	ListModules(ctx context.Context) ([]string, error)
}
