package resolvers

import "github.com/safran-ls/kors/kors-api/internal/usecase"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	RegisterResourceTypeUseCase *usecase.RegisterResourceTypeUseCase
	CreateResourceUseCase       *usecase.CreateResourceUseCase
	TransitionResourceUseCase   *usecase.TransitionResourceUseCase
	GrantPermissionUseCase      *usecase.GrantPermissionUseCase
}
