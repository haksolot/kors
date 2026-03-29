package core_test

import (
	"testing"

	"github.com/haksolot/kors/libs/core"
	"github.com/stretchr/testify/assert"
)

func TestClaims_HasRole(t *testing.T) {
	claims := &core.Claims{
		Roles: []string{core.RoleAdmin, core.RoleOperator},
	}

	assert.True(t, claims.HasRole(core.RoleAdmin))
	assert.True(t, claims.HasRole(core.RoleOperator))
	assert.False(t, claims.HasRole(core.RoleQualityManager))
}

func TestClaims_HasAnyRole(t *testing.T) {
	claims := &core.Claims{
		Roles: []string{core.RoleSupervisor},
	}

	assert.True(t, claims.HasAnyRole(core.RoleAdmin, core.RoleSupervisor))
	assert.True(t, claims.HasAnyRole(core.RoleSupervisor))
	assert.False(t, claims.HasAnyRole(core.RoleAdmin, core.RoleOperator))
}
