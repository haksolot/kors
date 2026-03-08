package resolvers

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/nats-io/nats.go"
	"github.com/haksolot/kors/kors-api/internal/adapter/postgres"
	korsnats "github.com/haksolot/kors/kors-api/internal/adapter/nats"
	korsminio "github.com/haksolot/kors/kors-api/internal/adapter/minio"
	"github.com/haksolot/kors/kors-api/internal/usecase"
	"github.com/haksolot/kors/kors-api/internal/domain/provisioning"
	"github.com/haksolot/kors/kors-api/internal/graph/model"
	"github.com/haksolot/kors/shared/korsctx"
	"github.com/rs/zerolog/log"
	"github.com/google/uuid"
	"context"
)

func getIdentityID(ctx context.Context) uuid.UUID {
	id, ok := korsctx.FromContext(ctx)
	if !ok {
		return uuid.Nil
	}
	return id
}

type Resolver struct {
	RegisterResourceTypeUseCase *usecase.RegisterResourceTypeUseCase
	CreateResourceUseCase       *usecase.CreateResourceUseCase
	TransitionResourceUseCase   *usecase.TransitionResourceUseCase
	GrantPermissionUseCase      *usecase.GrantPermissionUseCase
	CreateRevisionUseCase       *usecase.CreateRevisionUseCase
	ListResourcesUseCase        *usecase.ListResourcesUseCase
	ModuleGovernanceUseCase     *usecase.ModuleGovernanceUseCase
	UploadFileUseCase           *usecase.UploadFileUseCase
	GetResourceUseCase          *usecase.GetResourceUseCase
	GetResourceTypeUseCase      *usecase.GetResourceTypeUseCase
	CreateIdentityUseCase       *usecase.CreateIdentityUseCase
	GetIdentityUseCase          *usecase.GetIdentityUseCase // NOUVEAU
	ListIdentitiesUseCase       *usecase.ListIdentitiesUseCase // NOUVEAU
	DeleteResourceUseCase       *usecase.DeleteResourceUseCase
	ListEventsUseCase           *usecase.ListEventsUseCase // NOUVEAU
	ListPermissionsUseCase      *usecase.ListPermissionsUseCase // NOUVEAU
	IdentityRepo                *postgres.IdentityRepository // NOUVEAU
	NatsConn                    *nats.Conn
}

func NewResolver(pool *pgxpool.Pool, nc *nats.Conn, js nats.JetStreamContext, mClient *minio.Client) *Resolver {
	rtRepo := &postgres.ResourceTypeRepository{Pool: pool}
	rRepo := &postgres.ResourceRepository{Pool: pool}
	eRepo := &postgres.EventRepository{Pool: pool}
	pRepo := &postgres.PermissionRepository{Pool: pool}
	revRepo := &postgres.RevisionRepository{Pool: pool}
	idRepo := &postgres.IdentityRepository{Pool: pool}
	prov := &postgres.PostgresProvisioner{Pool: pool}
	storageProv := &korsminio.MinioProvisioner{Client: mClient}
	fStore := &korsminio.MinioFileStore{Client: mClient, DefaultBucket: "kors-files"}
	
	var ePub *korsnats.NatsPublisher
	if js != nil { ePub = &korsnats.NatsPublisher{JS: js} }

	return &Resolver{
		NatsConn: nc,
		RegisterResourceTypeUseCase: &usecase.RegisterResourceTypeUseCase{Repo: rtRepo, PermissionRepo: pRepo},
		CreateResourceUseCase:       &usecase.CreateResourceUseCase{Pool: pool, ResourceRepo: rRepo, ResourceTypeRepo: rtRepo, EventRepo: eRepo, PermissionRepo: pRepo, EventPublisher: ePub},
		TransitionResourceUseCase:   &usecase.TransitionResourceUseCase{Pool: pool, ResourceRepo: rRepo, ResourceTypeRepo: rtRepo, EventRepo: eRepo, PermissionRepo: pRepo, EventPublisher: ePub, Logger: log.Logger},
		GrantPermissionUseCase:      &usecase.GrantPermissionUseCase{Repo: pRepo, PermissionRepo: pRepo},
		CreateRevisionUseCase:       &usecase.CreateRevisionUseCase{ResourceRepo: rRepo, RevisionRepo: revRepo, FileStore: fStore, DefaultBucket: "kors-files", EventRepo: eRepo, EventPublisher: ePub},
		ListResourcesUseCase:        &usecase.ListResourcesUseCase{Repo: rRepo},
		ModuleGovernanceUseCase:     &usecase.ModuleGovernanceUseCase{Provisioner: prov, StorageProvisioner: storageProv, PermissionRepo: pRepo, IdentityRepo: idRepo},
		UploadFileUseCase:           &usecase.UploadFileUseCase{FileStore: fStore, IdentityRepo: idRepo},
		GetResourceUseCase:          &usecase.GetResourceUseCase{ResourceRepo: rRepo, PermissionRepo: pRepo},
		GetResourceTypeUseCase:      &usecase.GetResourceTypeUseCase{Repo: rtRepo},
		CreateIdentityUseCase:       &usecase.CreateIdentityUseCase{Repo: idRepo, PermissionRepo: pRepo},
		GetIdentityUseCase:          &usecase.GetIdentityUseCase{Repo: idRepo},
		ListIdentitiesUseCase:       &usecase.ListIdentitiesUseCase{Repo: idRepo, PermissionRepo: pRepo},
		DeleteResourceUseCase:       &usecase.DeleteResourceUseCase{ResourceRepo: rRepo, PermissionRepo: pRepo},
		ListEventsUseCase:           &usecase.ListEventsUseCase{Repo: eRepo},
		ListPermissionsUseCase:      &usecase.ListPermissionsUseCase{Repo: pRepo},
		IdentityRepo:                idRepo,
	}
}

func (r *Resolver) mapModuleRecordToInfo(ctx context.Context, rec *provisioning.ModuleRecord) (*model.ModuleInfo, error) {
	var ident *model.Identity
	if rec.IdentityID != nil {
		korsIdent, err := r.IdentityRepo.GetByID(ctx, *rec.IdentityID)
		if err == nil && korsIdent != nil {
			var extID *string
			if korsIdent.ExternalID != "" {
				extID = &korsIdent.ExternalID
			}
			ident = &model.Identity{
				ID:         korsIdent.ID,
				ExternalID: extID,
				Name:       korsIdent.Name,
				Type:       korsIdent.Type,
				Metadata:   korsIdent.Metadata,
				CreatedAt:  korsIdent.CreatedAt,
				UpdatedAt:  korsIdent.UpdatedAt,
			}
		}
	}

	return &model.ModuleInfo{
		Name:          rec.Name,
		SchemaName:    rec.SchemaName,
		PgUsername:    rec.PgUsername,
		BucketName:    rec.MinioBucket,
		Identity:      ident,
		ProvisionedAt: rec.ProvisionedAt,
	}, nil
}
