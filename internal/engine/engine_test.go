package engine

import (
	"context"
	"fmt"
	"github.com/1rp-pw/orchestrator/internal/policy"
	ConfigBuilder "github.com/keloran/go-config"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"testing"
	"time"
)

func TestSystem_RunPolicy(t *testing.T) {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "keloran/policy:latest",
		ExposedPorts: []string{"3000/tcp"},
		WaitingFor: wait.ForHTTP("/health").
			WithPort("3000/tcp").
			WithStartupTimeout(60 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	assert.NoError(t, err)

	endpoint, err := c.Endpoint(ctx, "")
	assert.NoError(t, err)

	cg := ConfigBuilder.NewConfigNoVault()
	cg.ProjectProperties = make(map[string]interface{})
	cg.ProjectProperties["engine_address"] = fmt.Sprintf("http://%s", endpoint)
	assert.NoError(t, err)

	s := NewSystem(cg)
	s.SetContext(ctx)

	p := policy.Policy{
		Data: map[string]map[string]interface{}{
			"Person": {
				"age":              18,
				"drivingTestScore": 60,
			},
		},
		Rule: `
        A **Person** gets a full driving license
        if the __age__ of the **Person** is greater than or equal to 17
        and the __drivingTestScore__ of the **Person** is greater than or equal to 60.
    `,
	}

	resp, err := s.RunPolicyInternal(p)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	testcontainers.CleanupContainer(t, c)
}
