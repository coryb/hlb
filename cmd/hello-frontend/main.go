package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/solver/errdefs"
	"github.com/moby/buildkit/util/appcontext"
)

func hello(name string) (*client.Result, error) {
	res := client.NewResult()
	if name == "" {
		name = "world"
	}
	res.Metadata = map[string][]byte{
		"greeting": []byte(fmt.Sprintf("hello %s", name)),
	}
	return res, nil
}

func main() {
	err := grpcclient.RunFromEnvironment(
		appcontext.Context(),
		func(ctx context.Context, c client.Client) (*client.Result, error) {
			opts := c.BuildOpts().Opts
			req, ok := opts["requestid"]
			if !ok {
				return nil, errors.New("missing requestid property")
			}
			switch req {
			case "hello-frontend.hello":
				return hello(opts["name"])
			default:
				return nil, errdefs.NewUnsupportedSubrequestError(req)
			}
		},
	)
	if err != nil {
		log.Fatal(err)
	}
}
