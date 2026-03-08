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
	isAdmin, err := uc.PermissionRepo.Check(ctx, input.CallerID, "admin", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	hasWrite, err := uc.PermissionRepo.Check(ctx, input.CallerID, "write", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	switch input.Type {
	case "user":
		// write or admin suffice to create a user
		if !hasWrite && !isAdmin {
			return nil, fmt.Errorf("write or admin permission required to create user identities")
		}
	case "service", "system":
		// only admin can create service or system identities
		if !isAdmin {
			return nil, fmt.Errorf("admin permission required to create %s identities", input.Type)
		}
	default:
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
