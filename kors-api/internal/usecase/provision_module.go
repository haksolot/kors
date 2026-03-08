package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/permission"
	"github.com/haksolot/kors/kors-api/internal/domain/provisioning"
)

type ModuleGovernanceUseCase struct {
	Provisioner    provisioning.Service
	PermissionRepo permission.Repository
}

func (uc *ModuleGovernanceUseCase) Provision(ctx context.Context, moduleName string, identityID uuid.UUID) (*provisioning.ModuleCredentials, error) {
	if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }
	return uc.Provisioner.ProvisionModule(ctx, moduleName)
}

func (uc *ModuleGovernanceUseCase) Deprovision(ctx context.Context, moduleName string, identityID uuid.UUID) error {
	if err := uc.checkAdmin(ctx, identityID); err != nil { return err }
	return uc.Provisioner.DeprovisionModule(ctx, moduleName)
}

func (uc *ModuleGovernanceUseCase) List(ctx context.Context, identityID uuid.UUID) ([]string, error) {
	if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }
	return uc.Provisioner.ListModules(ctx)
}

func (uc *ModuleGovernanceUseCase) checkAdmin(ctx context.Context, identityID uuid.UUID) error {
	allowed, err := uc.PermissionRepo.Check(ctx, identityID, "admin", nil, nil)
	if err != nil { return err }
	if !allowed {
		return fmt.Errorf("identity %s is not authorized for module governance", identityID)
	}
	return nil
}
