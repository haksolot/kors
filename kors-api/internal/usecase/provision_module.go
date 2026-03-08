package usecase

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/identity"
	"github.com/haksolot/kors/kors-api/internal/domain/permission"
	"github.com/haksolot/kors/kors-api/internal/domain/provisioning"
)

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

type ModuleGovernanceUseCase struct {
	Provisioner        provisioning.Service
	StorageProvisioner provisioning.StorageProvisioner
	PermissionRepo     permission.Repository
	IdentityRepo       identity.Repository
}

func (uc *ModuleGovernanceUseCase) grantModulePermissions(ctx context.Context, identID uuid.UUID) error {
	for _, action := range []string{"read", "write", "transition"} {
		err := uc.PermissionRepo.Create(ctx, &permission.Permission{
			ID:         uuid.New(),
			IdentityID: identID,
			// ResourceID and ResourceTypeID are nil => global scope
			Action:    action,
			CreatedAt: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("failed to grant %s permission to module: %w", action, err)
		}
	}
	return nil
}

func (uc *ModuleGovernanceUseCase) Provision(ctx context.Context, moduleName string, identityID uuid.UUID) (*provisioning.ModuleCredentials, error) {
	if err := validateModuleName(moduleName); err != nil { return nil, err }
	if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }

	// 1. Create or retrieve the module's KORS identity
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

	// 2. Grant KORS permissions to the module
	if err := uc.grantModulePermissions(ctx, ident.ID); err != nil {
		return nil, err
	}

	// 3. Provision MinIO
	if uc.StorageProvisioner != nil {
		if err := uc.StorageProvisioner.ProvisionBucket(ctx, moduleName); err != nil {
			return nil, fmt.Errorf("failed to provision storage: %w", err)
		}
	}

	// 4. Provision Postgres
	creds, err := uc.Provisioner.ProvisionModule(ctx, moduleName)
	if err != nil { return nil, err }

	// 5. Register in KORS registry
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
}

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

func (uc *ModuleGovernanceUseCase) Rotate(ctx context.Context, moduleName string, identityID uuid.UUID) (*provisioning.ModuleCredentials, error) {
	if err := validateModuleName(moduleName); err != nil { return nil, err }
	if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }
	return uc.Provisioner.RotatePassword(ctx, moduleName)
}

func (uc *ModuleGovernanceUseCase) Deprovision(ctx context.Context, moduleName string, identityID uuid.UUID, forceDeleteStorage bool) (*provisioning.DeprovisionReport, error) {
	if err := validateModuleName(moduleName); err != nil { return nil, err }
	if err := uc.checkAdmin(ctx, identityID); err != nil { return nil, err }

	report := &provisioning.DeprovisionReport{}

	// 1. Retrieve the module's KORS identity
	ident, _ := uc.IdentityRepo.GetByExternalID(ctx, moduleName)

	// 2. Remove KORS permissions for the module
	if ident != nil {
		if err := uc.PermissionRepo.DeleteForIdentity(ctx, ident.ID); err == nil {
			report.KorsDataCleared = true
		}
		// 3. Remove KORS identity
		_ = uc.IdentityRepo.Delete(ctx, ident.ID)
	} else {
		report.KorsDataCleared = true // nothing to clean
	}

	// 4. Deprovision MinIO
	if uc.StorageProvisioner != nil {
		err := uc.StorageProvisioner.DeprovisionBucket(ctx, moduleName, forceDeleteStorage)
		if err != nil {
			report.StorageSkippedReason = err.Error()
		} else {
			report.StorageCleared = true
		}
	}

	// 5. Deprovision Postgres
	if err := uc.Provisioner.DeprovisionModule(ctx, moduleName); err != nil {
		return report, fmt.Errorf("failed to deprovision postgres: %w", err)
	}
	report.PostgresCleared = true

	// 6. Remove from registry
	_ = uc.Provisioner.UnregisterModule(ctx, moduleName)

	report.Success = true
	return report, nil
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
