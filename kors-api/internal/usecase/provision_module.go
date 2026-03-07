package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/safran-ls/kors/kors-api/internal/domain/permission"
	"github.com/safran-ls/kors/kors-api/internal/domain/provisioning"
)

type ProvisionModuleUseCase struct {
	Provisioner    provisioning.Service
	PermissionRepo permission.Repository
}

func (uc *ProvisionModuleUseCase) Execute(ctx context.Context, moduleName string, identityID uuid.UUID) (*provisioning.ModuleCredentials, error) {
	// 1. Security Check
	allowed, err := uc.PermissionRepo.Check(ctx, identityID, "admin", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("permission check failed: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("identity %s is not authorized to provision modules", identityID)
	}

	// 2. Provisioning
	return uc.Provisioner.ProvisionModule(ctx, moduleName)
}
