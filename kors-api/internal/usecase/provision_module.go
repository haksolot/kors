package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/identity"
	"github.com/haksolot/kors/kors-api/internal/domain/permission"
	"github.com/haksolot/kors/kors-api/internal/domain/provisioning"
)

type ModuleGovernanceUseCase struct {
	Provisioner        provisioning.Service
	StorageProvisioner provisioning.StorageProvisioner
	PermissionRepo     permission.Repository
	IdentityRepo       identity.Repository
}

func (uc *ModuleGovernanceUseCase) Provision(ctx context.Context, moduleName string, identityID uuid.UUID) (*provisioning.ModuleCredentials, error) {
	if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }
	
	// Create KORS Identity for the module if not exists
	// externalID is moduleName for simple lookup
	ident, err := uc.IdentityRepo.GetByExternalID(ctx, moduleName)
	if err != nil { return nil, err }
	if ident == nil {
		err = uc.IdentityRepo.Create(ctx, &identity.Identity{
			ID:         uuid.New(),
			ExternalID: moduleName,
			Name:       moduleName,
			Type:       "service",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})
		if err != nil { return nil, fmt.Errorf("failed to create module identity: %w", err) }
	}

	// Provision MinIO Bucket
	if uc.StorageProvisioner != nil {
		if err := uc.StorageProvisioner.ProvisionBucket(ctx, moduleName); err != nil {
			return nil, fmt.Errorf("failed to provision storage: %w", err)
		}
	}
	
	return uc.Provisioner.ProvisionModule(ctx, moduleName)
}

func (uc *ModuleGovernanceUseCase) Deprovision(ctx context.Context, moduleName string, identityID uuid.UUID) error {
	if err := uc.checkAdmin(ctx, identityID); err != nil { return err }

	// Deprovision MinIO Bucket
	if uc.StorageProvisioner != nil {
		_ = uc.StorageProvisioner.DeprovisionBucket(ctx, moduleName)
	}

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
