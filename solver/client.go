package solver

import (
	"context"

	"github.com/moby/buildkit/client"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/pkg/errors"
)

type Client interface {
	gatewayClient() gateway.Client
	buildkitClient() *client.Client
}

func NewClientFromGateway(c gateway.Client) Client {
	return gatewayClientAdapter{c}
}

type gatewayClientAdapter struct {
	gateway.Client
}

func (g gatewayClientAdapter) gatewayClient() gateway.Client {
	return g.Client
}

func (g gatewayClientAdapter) buildkitClient() *client.Client {
	return nil
}

func NewClientFromBuildkit(c *client.Client) Client {
	return buildkitClientAdapter{c}
}

type buildkitClientAdapter struct {
	*client.Client
}

func (b buildkitClientAdapter) gatewayClient() gateway.Client {
	return nil
}

func (b buildkitClientAdapter) buildkitClient() *client.Client {
	return b.Client
}

// BuildkitClient returns a basic buildkit client.
func BuildkitClient(ctx context.Context, addr string) (Client, error) {
	opts := []client.ClientOpt{}
	cln, err := client.New(ctx, addr, opts...)
	if err != nil {
		return NewClientFromBuildkit(cln), err
	}
	_, err = cln.ListWorkers(ctx)
	return NewClientFromBuildkit(cln), errors.Wrap(err, "unable to connect to buildkitd")
}
