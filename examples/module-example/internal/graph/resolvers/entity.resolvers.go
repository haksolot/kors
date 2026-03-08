package resolvers

import (
	"context"

	"github.com/google/uuid"
	"github.com/haksolot/kors/examples/module-example/internal/graph/generated"
	"github.com/haksolot/kors/examples/module-example/internal/graph/model"
)

func (r *entityResolver) FindResourceByID(ctx context.Context, id uuid.UUID) (*model.Resource, error) {
	// 1. Chercher les données métier dans le schéma tms
	// On utilise l'id KORS comme clé primaire
	tool, err := r.Store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tool == nil {
		return nil, nil
	}

	// 2. Retourner l'objet étendu
	return &model.Resource{
		ID:       id,
		Diameter: &tool.Diameter,
		Length:   &tool.Length,
	}, nil
}

// Entity returns generated.EntityResolver implementation.
func (r *Resolver) Entity() generated.EntityResolver { return &entityResolver{r} }

type entityResolver struct{ *Resolver }
