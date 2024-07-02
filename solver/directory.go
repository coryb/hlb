package solver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/buildx/util/progress"
	"github.com/moby/buildkit/client/llb"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
	digest "github.com/opencontainers/go-digest"
	"github.com/openllb/hlb/parser"
	"github.com/openllb/hlb/parser/ast"
	"github.com/openllb/hlb/pkg/llbutil"
)

// NewRemoteDirectory returns an ast.Directory representing a directory backed
// by a BuildKit gateway reference.
func NewRemoteDirectory(ctx context.Context, cln Client, pw progress.Writer, def *llb.Definition, root string, dgst digest.Digest, solveOpts []SolveOption, sessionOpts []llbutil.SessionOption) (ast.Directory, error) {
	return &remoteDirectory{
		root:        root,
		dgst:        dgst,
		def:         def,
		cln:         cln,
		pw:          pw,
		solveOpts:   solveOpts,
		sessionOpts: sessionOpts,
		ctx:         ctx,
	}, nil
}

type remoteDirectory struct {
	root        string
	dgst        digest.Digest
	def         *llb.Definition
	cln         Client
	pw          progress.Writer
	solveOpts   []SolveOption
	sessionOpts []llbutil.SessionOption
	ctx         context.Context
}

func (r *remoteDirectory) Path() string {
	return r.root
}

func (r *remoteDirectory) Digest() digest.Digest {
	return r.dgst
}

func (r *remoteDirectory) Definition() *llb.Definition {
	return r.def
}

func (r *remoteDirectory) Open(filename string) (io.ReadCloser, error) {
	var data []byte
	if err := Build(r.ctx, r.cln, r.sessionOpts, func(ctx context.Context, c gateway.Client) (*gateway.Result, error) {
		dir, err := c.Solve(ctx, gateway.SolveRequest{
			Definition: r.def.ToPB(),
		})
		if err != nil {
			return nil, err
		}

		ref, err := dir.SingleRef()
		if err != nil {
			return nil, err
		}
		_, err = ref.StatFile(r.ctx, gateway.StatRequest{
			Path: filename,
		})
		if err != nil {
			return nil, err
		}

		data, err = ref.ReadFile(r.ctx, gateway.ReadRequest{
			Filename: filename,
		})
		if err != nil {
			return nil, err
		}
		return gateway.NewResult(), nil
	}, r.solveOpts...); err != nil {
		return nil, err
	}

	return &parser.NamedReader{
		Reader: bytes.NewReader(data),
		Value:  filepath.Join(r.root, filename),
	}, nil
}

// Stat is not called for remoteDirectory anywhere in the codebase, so
// here to satisfy the ast.Directory interface.
func (r *remoteDirectory) Stat(filename string) (os.FileInfo, error) {
	return nil, fmt.Errorf("stat on remote directory is unimplemented")
}
