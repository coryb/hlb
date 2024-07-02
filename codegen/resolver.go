package codegen

import (
	"context"
	"sync"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
	digest "github.com/opencontainers/go-digest"
	"github.com/openllb/hlb/parser/ast"
	"github.com/openllb/hlb/solver"
)

const (
	// ModuleFilename is the filename of the HLB module expected to be in the
	// solved filesystem provided to the import declaration.
	ModuleFilename = "module.hlb"
)

// Resolver resolves imports into a reader ready for parsing and checking.
type Resolver interface {
	// Resolve returns a reader for the HLB module and its compiled LLB.
	Resolve(ctx context.Context, id *ast.ImportDecl, fs Filesystem) (ast.Directory, error)
}

func NewCachedImageResolver(cln solver.Client) llb.ImageMetaResolver {
	return &cachedImageResolver{
		cln:   cln,
		cache: make(map[cacheKey]*imageConfig),
	}
}

type cacheKey struct {
	ref  string
	os   string
	arch string
}

type cachedImageResolver struct {
	cln   solver.Client
	cache map[cacheKey]*imageConfig
	mu    sync.RWMutex
}

type imageConfig struct {
	ref    string
	dgst   digest.Digest
	config []byte
}

func (r *cachedImageResolver) ResolveImageConfig(ctx context.Context, ref string, opt sourceresolver.Opt) (resolvedRef string, dgst digest.Digest, config []byte, err error) {
	key := cacheKey{ref: ref}
	if opt.Platform != nil {
		key.os = opt.Platform.OS
		key.arch = opt.Platform.Architecture
	}
	r.mu.RLock()
	cfg, ok := r.cache[key]
	r.mu.RUnlock()
	if ok {
		return cfg.ref, cfg.dgst, cfg.config, nil
	}

	err = solver.Build(ctx, r.cln, nil, func(ctx context.Context, c gateway.Client) (res *gateway.Result, err error) {
		resolvedRef, dgst, config, err = c.ResolveImageConfig(ctx, ref, opt)
		return gateway.NewResult(), err
	})
	if err != nil {
		return
	}

	r.mu.Lock()
	r.cache[key] = &imageConfig{
		ref:    resolvedRef,
		dgst:   dgst,
		config: config,
	}
	r.mu.Unlock()
	return
}
