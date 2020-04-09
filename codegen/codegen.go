package codegen

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/docker/buildx/util/progress"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	"github.com/openllb/hlb/checker"
	"github.com/openllb/hlb/parser"
	"github.com/openllb/hlb/solver"
	"github.com/palantir/stacktrace"
)

type CodeGen struct {
	Debug     Debugger
	request   solver.Request
	mw        *progress.MultiWriter
	dockerCli *command.DockerCli
	solveOpts []solver.SolveOption
}

type CodeGenOption func(*CodeGen) error

func WithDebugger(dbgr Debugger) CodeGenOption {
	return func(i *CodeGen) error {
		i.Debug = dbgr
		return nil
	}
}

func WithMultiWriter(mw *progress.MultiWriter) CodeGenOption {
	return func(i *CodeGen) error {
		i.mw = mw
		return nil
	}
}

func New(opts ...CodeGenOption) (*CodeGen, error) {
	cg := &CodeGen{
		Debug: NewNoopDebugger(),
	}
	for _, opt := range opts {
		err := opt(cg)
		if err != nil {
			return cg, stacktrace.Propagate(err, "")
		}
	}

	return cg, nil
}

func (cg *CodeGen) SolveOptions() []solver.SolveOption {
	return cg.solveOpts
}

func (cg *CodeGen) Generate(ctx context.Context, mod *parser.Module, targets []*parser.CallStmt) (solver.Request, error) {
	cg.request = solver.NewEmptyRequest()

	for _, target := range targets {
		// Reset solve options for this target.
		// If we need to paralellize compilation then we can revisit this.
		cg.solveOpts = []solver.SolveOption{}

		obj := mod.Scope.Lookup(target.Func.Ident.Name)
		if obj == nil {
			return cg.request, fmt.Errorf("unknown target %q", target.Func.Ident.Name)
		}

		// Yield to the debugger before compiling anything.
		err := cg.Debug(ctx, mod.Scope, mod, nil)
		if err != nil {
			return cg.request, stacktrace.Propagate(err, "")
		}

		var st llb.State
		switch obj.Kind {
		case parser.DeclKind:
			switch n := obj.Node.(type) {
			case *parser.FuncDecl:
				if n.Type.ObjType != parser.Filesystem {
					return cg.request, checker.ErrInvalidTarget{Ident: target.Func.Ident}
				}

				st, err = cg.EmitFilesystemFuncDecl(ctx, mod.Scope, n, target, noopAliasCallback)
				if err != nil {
					return cg.request, stacktrace.Propagate(err, "")
				}
			case *parser.AliasDecl:
				if n.Func.Type.ObjType != parser.Filesystem {
					return cg.request, checker.ErrInvalidTarget{Ident: target.Func.Ident}
				}

				st, err = cg.EmitFilesystemAliasDecl(ctx, mod.Scope, n, target)
				if err != nil {
					return cg.request, stacktrace.Propagate(err, "")
				}
			}
		default:
			return cg.request, checker.ErrInvalidTarget{Ident: target.Func.Ident}
		}

		cg.request = cg.request.Peer(solver.NewRequest(st, cg.solveOpts...))
	}

	return cg.request, nil
}

func (cg *CodeGen) GenerateImport(ctx context.Context, scope *parser.Scope, lit *parser.FuncLit) (llb.State, error) {
	return cg.EmitFilesystemBlock(ctx, scope, lit.Body.NonEmptyStmts(), nil)
}

type aliasCallback func(*parser.CallStmt, interface{}) bool

func noopAliasCallback(_ *parser.CallStmt, _ interface{}) bool { return true }

func isBreakpoint(call *parser.CallStmt) bool {
	return call.Func.Ident != nil && call.Func.Ident.Name == "breakpoint"
}

func (cg *CodeGen) EmitBlock(ctx context.Context, scope *parser.Scope, typ parser.ObjType, stmts []*parser.Stmt, ac aliasCallback) (interface{}, error) {
	var v interface{}
	switch typ {
	case parser.Filesystem:
		v = llb.Scratch()
	case parser.Str:
		v = ""
	}

	var err error
	for _, stmt := range stmts {
		call := stmt.Call
		if isBreakpoint(call) {
			err = cg.Debug(ctx, scope, call, v)
			if err != nil {
				return nil, stacktrace.Propagate(err, "")
			}
			continue
		}

		// Before executing the next call statement.
		err = cg.Debug(ctx, scope, call, v)
		if err != nil {
			return nil, stacktrace.Propagate(err, "")
		}

		chain, err := cg.EmitChainStmt(ctx, scope, typ, call, ac)
		if err != nil {
			return nil, stacktrace.Propagate(err, "")
		}
		v = chain(v)

		if st, ok := v.(llb.State); ok && st.Output() != nil {
			err = st.Validate()
			if err != nil {
				return nil, checker.ErrCodeGen{Node: stmt, Err: err}
			}
		}

		if call.Alias != nil {
			// Chain statements may be aliased.
			cont := ac(call, v)
			if !cont {
				break
			}
		}
	}

	return v, nil
}

func (cg *CodeGen) EmitChainStmt(ctx context.Context, scope *parser.Scope, typ parser.ObjType, call *parser.CallStmt, ac aliasCallback) (func(v interface{}) interface{}, error) {
	switch typ {
	case parser.Filesystem:
		chain, err := cg.EmitFilesystemChainStmt(ctx, scope, typ, call, ac)
		if err != nil {
			return nil, stacktrace.Propagate(err, "")
		}
		return func(v interface{}) interface{} {
			return chain(v.(llb.State))
		}, nil
	case parser.Str:
		chain, err := cg.EmitStringChainStmt(ctx, scope, call)
		if err != nil {
			return nil, stacktrace.Propagate(err, "")
		}
		return func(v interface{}) interface{} {
			return chain(v.(string))
		}, nil
	default:
		panic("unknown chain stmt")
	}
}

func (cg *CodeGen) EmitStringChainStmt(ctx context.Context, scope *parser.Scope, call *parser.CallStmt) (func(string) string, error) {
	args := call.Args
	name := call.Func.Ident.Name
	switch name {
	case "value":
		val, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		return func(_ string) string {
			return val
		}, err
	case "format":
		formatStr, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return nil, stacktrace.Propagate(err, "")
		}

		var as []interface{}
		for _, arg := range args[1:] {
			a, err := cg.EmitStringExpr(ctx, scope, call, arg)
			if err != nil {
				return nil, stacktrace.Propagate(err, "")
			}
			as = append(as, a)
		}

		return func(_ string) string {
			return fmt.Sprintf(formatStr, as...)
		}, nil
	default:
		// must be a named reference
		obj := scope.Lookup(name)
		if obj == nil {
			panic(name)
		}

		var v interface{}
		var err error
		switch n := obj.Node.(type) {
		case *parser.FuncDecl:
			v, err = cg.EmitFuncDecl(ctx, scope, n, call, "", noopAliasCallback)
		case *parser.AliasDecl:
			v, err = cg.EmitAliasDecl(ctx, scope, n, call)
		case *parser.ImportDecl:
			importScope := obj.Data.(*parser.Scope)
			importObj := importScope.Lookup(call.Func.Selector.Select.Name)
			switch m := importObj.Node.(type) {
			case *parser.FuncDecl:
				v, err = cg.EmitFuncDecl(ctx, scope, m, call, "", noopAliasCallback)
			case *parser.AliasDecl:
				v, err = cg.EmitAliasDecl(ctx, scope, m, call)
			default:
				panic("unknown obj type")
			}
		case *parser.Field:
			v = obj.Data
		default:
			panic("unknown obj type")
		}
		if err != nil {
			return nil, stacktrace.Propagate(err, "")
		}
		return func(_ string) string {
			return v.(string)
		}, nil
	}
}

func (cg *CodeGen) EmitFilesystemBlock(ctx context.Context, scope *parser.Scope, stmts []*parser.Stmt, ac aliasCallback) (llb.State, error) {
	v, err := cg.EmitBlock(ctx, scope, parser.Filesystem, stmts, ac)
	if err != nil {
		return llb.Scratch(), stacktrace.Propagate(err, "")
	}
	return v.(llb.State), nil
}

func (cg *CodeGen) EmitStringBlock(ctx context.Context, scope *parser.Scope, stmts []*parser.Stmt) (string, error) {
	v, err := cg.EmitBlock(ctx, scope, parser.Str, stmts, noopAliasCallback)
	if err != nil {
		return "", stacktrace.Propagate(err, "")
	}
	return v.(string), nil
}

func (cg *CodeGen) EmitFuncLit(ctx context.Context, scope *parser.Scope, lit *parser.FuncLit, op string, ac aliasCallback) (interface{}, error) {
	switch lit.Type.Primary() {
	case parser.Int, parser.Bool:
		panic("unimplemented")
	case parser.Filesystem:
		return cg.EmitFilesystemBlock(ctx, scope, lit.Body.NonEmptyStmts(), ac)
	case parser.Str:
		return cg.EmitStringBlock(ctx, scope, lit.Body.NonEmptyStmts())
	case parser.Option:
		return cg.EmitOptions(ctx, scope, op, lit.Body.NonEmptyStmts(), ac)
	}
	return nil, nil
}

func (cg *CodeGen) EmitWithOption(ctx context.Context, scope *parser.Scope, parent *parser.CallStmt, with *parser.WithOpt, ac aliasCallback) ([]interface{}, error) {
	if with == nil {
		return nil, nil
	}

	switch {
	case with.Ident != nil:
		obj := scope.Lookup(with.Ident.Name)
		switch obj.Kind {
		case parser.ExprKind:
			return obj.Data.([]interface{}), nil
		case parser.DeclKind:
			if n, ok := obj.Node.(*parser.FuncDecl); ok {
				return cg.EmitOptions(ctx, scope, parent.Func.Ident.Name, n.Body.NonEmptyStmts(), ac)
			} else {
				panic("unknown decl type")
			}
		default:
			panic("unknown with option kind")
		}
	case with.FuncLit != nil:
		return cg.EmitOptions(ctx, scope, parent.Func.Ident.Name, with.FuncLit.Body.NonEmptyStmts(), ac)
	default:
		panic("unknown with option")
	}
}

func (cg *CodeGen) EmitFilesystemChainStmt(ctx context.Context, scope *parser.Scope, typ parser.ObjType, call *parser.CallStmt, ac aliasCallback) (so llb.StateOption, err error) {
	args := call.Args
	iopts, err := cg.EmitWithOption(ctx, scope, call, call.WithOpt, ac)
	if err != nil {
		return so, stacktrace.Propagate(err, "")
	}

	var name string
	switch {
	case call.Func.Ident != nil:
		name = call.Func.Ident.Name
	case call.Func.Selector != nil:
		name = call.Func.Selector.Ident.Name
	}

	switch name {
	case "scratch":
		so = func(_ llb.State) llb.State {
			return llb.Scratch()
		}
	case "image":
		ref, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		var opts []llb.ImageOption
		for _, iopt := range iopts {
			opt := iopt.(llb.ImageOption)
			opts = append(opts, opt)
		}

		so = func(_ llb.State) llb.State {
			return llb.Image(ref, opts...)
		}
	case "http":
		url, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		var opts []llb.HTTPOption
		for _, iopt := range iopts {
			opt := iopt.(llb.HTTPOption)
			opts = append(opts, opt)
		}

		so = func(_ llb.State) llb.State {
			return llb.HTTP(url, opts...)
		}
	case "git":
		remote, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}
		ref, err := cg.EmitStringExpr(ctx, scope, call, args[1])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		var opts []llb.GitOption
		for _, iopt := range iopts {
			opt := iopt.(llb.GitOption)
			opts = append(opts, opt)
		}
		so = func(_ llb.State) llb.State {
			return llb.Git(remote, ref, opts...)
		}
	case "local":
		path, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		var opts []llb.LocalOption
		for _, iopt := range iopts {
			opt := iopt.(llb.LocalOption)
			opts = append(opts, opt)
		}

		// Get a consistent hash for this local (path + options) so we don't
		// transport the same content multiple times when referenced repeatedly.

		// First get serialized bytes for this llb.Local state.
		tmpSt := llb.Local("", opts...)
		_, hashInput, _, err := tmpSt.Output().Vertex().Marshal(&llb.Constraints{})
		if err != nil {
			return so, checker.ErrCodeGen{Node: call, Err: err}
		}

		// Next append the path so we have the path + options serialized hash input.
		hashInput = append(hashInput, []byte(path)...)

		id := string(digest.FromBytes(hashInput))
		cg.solveOpts = append(cg.solveOpts, solver.WithLocal(id, path))

		so = func(_ llb.State) llb.State {
			return llb.Local(id, opts...)
		}
	case "generate":
		frontend, err := cg.EmitFilesystemExpr(ctx, scope, args[0], ac)
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		opts := []llb.FrontendOption{llb.IgnoreCache}
		for _, iopt := range iopts {
			opt := iopt.(llb.FrontendOption)
			opts = append(opts, opt)
		}

		so = func(_ llb.State) llb.State {
			return llb.Frontend(frontend, opts...)
		}
	case "run":
		var shlex string
		if len(args) == 1 {
			commandStr, err := cg.EmitStringExpr(ctx, scope, call, args[0])
			if err != nil {
				return so, stacktrace.Propagate(err, "")
			}

			parts, err := shellquote.Split(commandStr)
			if err != nil {
				return so, stacktrace.Propagate(err, "")
			}

			if len(parts) == 1 {
				shlex = commandStr
			} else {
				shlex = shellquote.Join("/bin/sh", "-c", commandStr)
			}
		} else {
			var runArgs []string
			for _, arg := range args {
				runArg, err := cg.EmitStringExpr(ctx, scope, call, arg)
				if err != nil {
					return so, stacktrace.Propagate(err, "")
				}
				runArgs = append(runArgs, runArg)
			}
			shlex = shellquote.Join(runArgs...)
		}

		var opts []llb.RunOption
		for _, iopt := range iopts {
			opt := iopt.(llb.RunOption)
			opts = append(opts, opt)
		}

		var targets []string
		calls := make(map[string]*parser.CallStmt)

		with := call.WithOpt
		if with != nil {
			switch {
			case with.Ident != nil:
				// Do nothing.
				//
				// Mounts inside option functions cannot be aliased because they need
				// to be in the context of a specific function run is in.
			case with.FuncLit != nil:
				for _, stmt := range with.FuncLit.Body.NonEmptyStmts() {
					if stmt.Call.Func.Ident.Name != "mount" || stmt.Call.Alias == nil {
						continue
					}

					target, err := cg.EmitStringExpr(ctx, scope, call, stmt.Call.Args[1])
					if err != nil {
						return so, stacktrace.Propagate(err, "")
					}

					calls[target] = stmt.Call
					targets = append(targets, target)
				}
			default:
				panic("unknown with option")
			}
		}

		opts = append(opts, llb.Shlex(shlex))
		so = func(st llb.State) llb.State {
			exec := st.Run(opts...)

			if len(targets) > 0 {
				for _, target := range targets {
					// Mounts are unique by its mountpoint, and its vertex representing the
					// mount after execing can be aliased.
					cont := ac(calls[target], exec.GetMount(target))
					if !cont {
						break
					}
				}
			}

			return exec.Root()
		}
	case "env":
		key, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		value, err := cg.EmitStringExpr(ctx, scope, call, args[1])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		so = func(st llb.State) llb.State {
			return st.AddEnv(key, value)
		}
	case "dir":
		path, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		so = func(st llb.State) llb.State {
			return st.Dir(path)
		}
	case "user":
		name, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		so = func(st llb.State) llb.State {
			return st.User(name)
		}
	case "entrypoint":
		var stArgs []string
		for _, arg := range args {
			stArg, err := cg.EmitStringExpr(ctx, scope, call, arg)
			if err != nil {
				return so, stacktrace.Propagate(err, "")
			}
			stArgs = append(stArgs, stArg)
		}

		so = func(st llb.State) llb.State {
			return st.Args(stArgs...)
		}
	case "mkdir":
		path, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		mode, err := cg.EmitIntExpr(ctx, scope, args[1])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		iopts, err := cg.EmitWithOption(ctx, scope, call, call.WithOpt, ac)
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		var opts []llb.MkdirOption
		for _, iopt := range iopts {
			opt := iopt.(llb.MkdirOption)
			opts = append(opts, opt)
		}

		so = func(st llb.State) llb.State {
			return st.File(
				llb.Mkdir(path, os.FileMode(mode), opts...),
			)
		}
	case "mkfile":
		path, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		mode, err := cg.EmitIntExpr(ctx, scope, args[1])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		content, err := cg.EmitStringExpr(ctx, scope, call, args[2])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		var opts []llb.MkfileOption
		for _, iopt := range iopts {
			opt := iopt.(llb.MkfileOption)
			opts = append(opts, opt)
		}

		so = func(st llb.State) llb.State {
			return st.File(
				llb.Mkfile(path, os.FileMode(mode), []byte(content), opts...),
			)
		}
	case "rm":
		path, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		var opts []llb.RmOption
		for _, iopt := range iopts {
			opt := iopt.(llb.RmOption)
			opts = append(opts, opt)
		}

		so = func(st llb.State) llb.State {
			return st.File(
				llb.Rm(path, opts...),
			)
		}
	case "copy":
		input, err := cg.EmitFilesystemExpr(ctx, scope, args[0], ac)
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		src, err := cg.EmitStringExpr(ctx, scope, call, args[1])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		dest, err := cg.EmitStringExpr(ctx, scope, call, args[2])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		var opts []llb.CopyOption
		for _, iopt := range iopts {
			opt := iopt.(llb.CopyOption)
			opts = append(opts, opt)
		}

		so = func(st llb.State) llb.State {
			return st.File(
				llb.Copy(input, src, dest, opts...),
			)
		}
	case "dockerPush":
		ref, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}
		so = func(st llb.State) llb.State {
			solveOpts := append(cg.solveOpts, solver.WithPushImage(ref))
			for _, iopt := range iopts {
				opt := iopt.(solver.SolveOption)
				solveOpts = append(solveOpts, opt)
			}
			cg.request = cg.request.Peer(solver.NewRequest(st, solveOpts...))
			return st
		}
	case "dockerLoad":
		ref, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}
		if cg.mw == nil {
			return so, fmt.Errorf("progress.MultiWriter must be provided for dockerLoad")
		}

		if cg.dockerCli == nil {
			cg.dockerCli, err = command.NewDockerCli()
			if err != nil {
				return so, stacktrace.Propagate(err, "")
			}

			err = cg.dockerCli.Initialize(flags.NewClientOptions())
			if err != nil {
				return so, stacktrace.Propagate(err, "")
			}
		}

		r, w := io.Pipe()
		done := make(chan struct{})
		cg.solveOpts = append(cg.solveOpts, solver.WithWaiter(done))

		go func() {
			defer close(done)

			resp, err := cg.dockerCli.Client().ImageLoad(ctx, r, true)
			if err != nil {
				r.CloseWithError(err)
				return
			}
			defer resp.Body.Close()

			pw := cg.mw.WithPrefix("", false)
			progress.FromReader(pw, fmt.Sprintf("importing %s to docker", ref), resp.Body)
		}()

		so = func(st llb.State) llb.State {
			solveOpts := append(cg.solveOpts, solver.WithDownloadDockerTarball(ref, w))
			for _, iopt := range iopts {
				opt := iopt.(solver.SolveOption)
				solveOpts = append(solveOpts, opt)
			}
			cg.request = cg.request.Peer(solver.NewRequest(st, solveOpts...))
			return st
		}

	case "download":
		localPath, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		so = func(st llb.State) llb.State {
			solveOpts := append(cg.solveOpts, solver.WithDownload(localPath))
			for _, iopt := range iopts {
				opt := iopt.(solver.SolveOption)
				solveOpts = append(solveOpts, opt)
			}

			cg.request = cg.request.Peer(solver.NewRequest(st, solveOpts...))
			return st
		}
	case "downloadTarball":
		localPath, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		f, err := os.Open(localPath)
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		so = func(st llb.State) llb.State {
			solveOpts := append(cg.solveOpts, solver.WithDownloadTarball(f))
			for _, iopt := range iopts {
				opt := iopt.(solver.SolveOption)
				solveOpts = append(solveOpts, opt)
			}
			cg.request = cg.request.Peer(solver.NewRequest(st, solveOpts...))
			return st
		}
	case "downloadOCITarball":
		localPath, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		f, err := os.Open(localPath)
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		so = func(st llb.State) llb.State {
			solveOpts := append(cg.solveOpts, solver.WithDownloadOCITarball(f))
			for _, iopt := range iopts {
				opt := iopt.(solver.SolveOption)
				solveOpts = append(solveOpts, opt)
			}
			cg.request = cg.request.Peer(solver.NewRequest(st, solveOpts...))
			return st
		}
	case "downloadDockerTarball":
		localPath, err := cg.EmitStringExpr(ctx, scope, call, args[0])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		f, err := os.Open(localPath)
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		ref, err := cg.EmitStringExpr(ctx, scope, call, args[1])
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}

		so = func(st llb.State) llb.State {
			solveOpts := append(cg.solveOpts, solver.WithDownloadDockerTarball(ref, f))
			for _, iopt := range iopts {
				opt := iopt.(solver.SolveOption)
				solveOpts = append(solveOpts, opt)
			}
			cg.request = cg.request.Peer(solver.NewRequest(st, solveOpts...))
			return st
		}
	default:
		// must be a named reference
		obj := scope.Lookup(name)
		if obj == nil {
			panic(name)
		}

		var v interface{}
		var err error
		switch n := obj.Node.(type) {
		case *parser.FuncDecl:
			v, err = cg.EmitFuncDecl(ctx, scope, n, call, "", ac)
		case *parser.AliasDecl:
			v, err = cg.EmitAliasDecl(ctx, scope, n, call)
		case *parser.ImportDecl:
			importScope := obj.Data.(*parser.Scope)
			importObj := importScope.Lookup(call.Func.Selector.Select.Name)
			switch m := importObj.Node.(type) {
			case *parser.FuncDecl:
				v, err = cg.EmitFuncDecl(ctx, scope, m, call, "", ac)
			case *parser.AliasDecl:
				v, err = cg.EmitAliasDecl(ctx, scope, m, call)
			default:
				panic("unknown obj type")
			}
		case *parser.Field:
			v = obj.Data
		default:
			panic("unknown obj type")
		}
		if err != nil {
			return so, stacktrace.Propagate(err, "")
		}
		so = func(_ llb.State) llb.State {
			return v.(llb.State)
		}
	}

	return so, nil
}

func (cg *CodeGen) EmitOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt, ac aliasCallback) ([]interface{}, error) {
	switch op {
	case "image":
		return cg.EmitImageOptions(ctx, scope, op, stmts)
	case "http":
		return cg.EmitHTTPOptions(ctx, scope, op, stmts)
	case "git":
		return cg.EmitGitOptions(ctx, scope, op, stmts)
	case "local":
		return cg.EmitLocalOptions(ctx, scope, op, stmts)
	case "generate":
		return cg.EmitGenerateOptions(ctx, scope, op, stmts, ac)
	case "run":
		return cg.EmitExecOptions(ctx, scope, op, stmts, ac)
	case "ssh":
		return cg.EmitSSHOptions(ctx, scope, op, stmts)
	case "secret":
		return cg.EmitSecretOptions(ctx, scope, op, stmts)
	case "mount":
		return cg.EmitMountOptions(ctx, scope, op, stmts)
	case "mkdir":
		return cg.EmitMkdirOptions(ctx, scope, op, stmts)
	case "mkfile":
		return cg.EmitMkfileOptions(ctx, scope, op, stmts)
	case "rm":
		return cg.EmitRmOptions(ctx, scope, op, stmts)
	case "copy":
		return cg.EmitCopyOptions(ctx, scope, op, stmts)
	default:
		panic("call stmt does not support options")
	}
}

func (cg *CodeGen) EmitImageOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "resolve":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if v {
					opts = append(opts, imagemetaresolver.WithDefault)
				}
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}
	return
}

func (cg *CodeGen) EmitHTTPOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "checksum":
				dgst, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.Checksum(digest.Digest(dgst)))
			case "chmod":
				mode, err := cg.EmitIntExpr(ctx, scope, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.Chmod(os.FileMode(mode)))
			case "filename":
				filename, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.Filename(filename))
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}
	return
}

func (cg *CodeGen) EmitGitOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "keepGitDir":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if v {
					opts = append(opts, llb.KeepGitDir())
				}
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}
	return
}

func (cg *CodeGen) EmitLocalOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "includePatterns":
				patterns := make([]string, len(args))
				for i, arg := range args {
					patterns[i], err = cg.EmitStringExpr(ctx, scope, stmt.Call, arg)
					if err != nil {
						return opts, stacktrace.Propagate(err, "")
					}
				}
				opts = append(opts, llb.IncludePatterns(patterns))
			case "excludePatterns":
				patterns := make([]string, len(args))
				for i, arg := range args {
					patterns[i], err = cg.EmitStringExpr(ctx, scope, stmt.Call, arg)
					if err != nil {
						return opts, stacktrace.Propagate(err, "")
					}
				}
				opts = append(opts, llb.ExcludePatterns(patterns))
			case "followPaths":
				paths := make([]string, len(args))
				for i, arg := range args {
					paths[i], err = cg.EmitStringExpr(ctx, scope, stmt.Call, arg)
					if err != nil {
						return opts, stacktrace.Propagate(err, "")
					}
				}
				opts = append(opts, llb.FollowPaths(paths))
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}
	return
}

func (cg *CodeGen) EmitGenerateOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt, ac aliasCallback) (opts []interface{}, err error) {
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "frontendInput":
				key, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				value, err := cg.EmitFilesystemExpr(ctx, scope, args[1], ac)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.WithFrontendInput(key, value))
			case "frontendOpt":
				key, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				value, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[1])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.WithFrontendOpt(key, value))
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}
	return
}

func (cg *CodeGen) EmitMkdirOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "createParents":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.WithParents(v))
			case "chown":
				owner, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.WithUser(owner))
			case "createdTime":
				v, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				t, err := time.Parse(time.RFC3339, v)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				opts = append(opts, llb.WithCreatedTime(t))
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}
	return
}

func (cg *CodeGen) EmitMkfileOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "chown":
				owner, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.WithUser(owner))
			case "createdTime":
				v, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				t, err := time.Parse(time.RFC3339, v)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				opts = append(opts, llb.WithCreatedTime(t))
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}
	return
}

func (cg *CodeGen) EmitRmOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "allowNotFound":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.WithAllowNotFound(v))
			case "allowWildcard":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.WithAllowWildcard(v))
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}
	return
}

func (cg *CodeGen) EmitCopyOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	cp := &llb.CopyInfo{}

	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "followSymlinks":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				cp.FollowSymlinks = v
			case "contentsOnly":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				cp.CopyDirContentsOnly = v
			case "unpack":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				cp.AttemptUnpack = v
			case "createDestPath":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				cp.CreateDestPath = v
			case "allowWildcards":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				cp.AllowWildcard = v
			case "allowEmptyWildcard":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				cp.AllowEmptyWildcard = v
			case "chown":
				owner, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.WithUser(owner))
			case "createdTime":
				v, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				t, err := time.Parse(time.RFC3339, v)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				opts = append(opts, llb.WithCreatedTime(t))
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}

	opts = append([]interface{}{cp}, opts...)
	return
}

func (cg *CodeGen) EmitExecOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt, ac aliasCallback) (opts []interface{}, err error) {
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			iopts, err := cg.EmitWithOption(ctx, scope, stmt.Call, stmt.Call.WithOpt, ac)
			if err != nil {
				return opts, stacktrace.Propagate(err, "")
			}

			switch stmt.Call.Func.Ident.Name {
			case "readonlyRootfs":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if v {
					opts = append(opts, llb.ReadonlyRootFS())
				}
			case "env":
				key, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				value, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[1])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				opts = append(opts, llb.AddEnv(key, value))
			case "dir":
				path, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				opts = append(opts, llb.Dir(path))
			case "user":
				name, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				opts = append(opts, llb.User(name))
			case "network":
				mode, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				var netMode pb.NetMode
				switch mode {
				case "unset":
					netMode = pb.NetMode_UNSET
				case "host":
					netMode = pb.NetMode_HOST
				case "node":
					netMode = pb.NetMode_NONE
				default:
					panic("unknown network mode")
				}

				opts = append(opts, llb.Network(netMode))
			case "security":
				mode, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				var securityMode pb.SecurityMode
				switch mode {
				case "sandbox":
					securityMode = pb.SecurityMode_SANDBOX
				case "insecure":
					securityMode = pb.SecurityMode_INSECURE
				default:
					panic("unknown network mode")
				}

				opts = append(opts, llb.Security(securityMode))
			case "host":
				host, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				address, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[1])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				ip := net.ParseIP(address)

				opts = append(opts, llb.AddExtraHost(host, ip))
			case "ssh":
				var sshOpts []llb.SSHOption
				for _, iopt := range iopts {
					opt := iopt.(llb.SSHOption)
					sshOpts = append(sshOpts, opt)
				}

				opts = append(opts, llb.AddSSHSocket(sshOpts...))
			case "secret":
				localPath, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				mountPoint, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[1])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				id := string(digest.FromString(localPath))
				cg.solveOpts = append(cg.solveOpts, solver.WithSecret(id, localPath))

				secretOpts := []llb.SecretOption{
					llb.SecretID(id),
				}
				for _, iopt := range iopts {
					opt := iopt.(llb.SecretOption)
					secretOpts = append(secretOpts, opt)
				}

				opts = append(opts, llb.AddSecret(mountPoint, secretOpts...))
			case "mount":
				input, err := cg.EmitFilesystemExpr(ctx, scope, args[0], ac)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				target, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[1])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				var mountOpts []llb.MountOption
				for _, iopt := range iopts {
					opt := iopt.(llb.MountOption)
					mountOpts = append(mountOpts, opt)
				}

				opts = append(opts, llb.AddMount(target, input, mountOpts...))
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}
	return
}

type sshSocketOpt struct {
	target string
	uid    int
	gid    int
	mode   os.FileMode
}

func (cg *CodeGen) EmitSSHOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	var sopt *sshSocketOpt
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "target":
				target, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if sopt == nil {
					sopt = &sshSocketOpt{}
				}
				sopt.target = target
			case "id":
				id, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.SSHID(id))
			case "uid":
				uid, err := cg.EmitIntExpr(ctx, scope, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if sopt == nil {
					sopt = &sshSocketOpt{}
				}
				sopt.uid = uid
			case "gid":
				gid, err := cg.EmitIntExpr(ctx, scope, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if sopt == nil {
					sopt = &sshSocketOpt{}
				}
				sopt.gid = gid
			case "mode":
				mode, err := cg.EmitIntExpr(ctx, scope, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if sopt == nil {
					sopt = &sshSocketOpt{}
				}
				sopt.mode = os.FileMode(mode)
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}

	if sopt != nil {
		opts = append(opts, llb.SSHSocketOpt(
			sopt.target,
			sopt.uid,
			sopt.gid,
			int(sopt.mode),
		))
	}

	return
}

type secretOpt struct {
	uid  int
	gid  int
	mode os.FileMode
}

func (cg *CodeGen) EmitSecretOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	var sopt *secretOpt
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "id":
				id, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.SecretID(id))
			case "uid":
				uid, err := cg.EmitIntExpr(ctx, scope, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if sopt == nil {
					sopt = &secretOpt{}
				}
				sopt.uid = uid
			case "gid":
				gid, err := cg.EmitIntExpr(ctx, scope, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if sopt == nil {
					sopt = &secretOpt{}
				}
				sopt.gid = gid
			case "mode":
				mode, err := cg.EmitIntExpr(ctx, scope, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if sopt == nil {
					sopt = &secretOpt{}
				}
				sopt.mode = os.FileMode(mode)
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}

	if sopt != nil {
		opts = append(opts, llb.SecretFileOpt(
			sopt.uid,
			sopt.gid,
			int(sopt.mode),
		))
	}

	return
}

func (cg *CodeGen) EmitMountOptions(ctx context.Context, scope *parser.Scope, op string, stmts []*parser.Stmt) (opts []interface{}, err error) {
	for _, stmt := range stmts {
		if stmt.Call != nil {
			args := stmt.Call.Args
			switch stmt.Call.Func.Ident.Name {
			case "readonly":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if v {
					opts = append(opts, llb.MountOption(llb.Readonly))
				}
			case "tmpfs":
				v, err := cg.MaybeEmitBoolExpr(ctx, scope, args)
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				if v {
					opts = append(opts, llb.Tmpfs())
				}
			case "sourcePath":
				path, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, llb.SourcePath(path))
			case "cache":
				id, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[0])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				mode, err := cg.EmitStringExpr(ctx, scope, stmt.Call, args[1])
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}

				var sharing llb.CacheMountSharingMode
				switch mode {
				case "shared":
					sharing = llb.CacheMountShared
				case "private":
					sharing = llb.CacheMountPrivate
				case "locked":
					sharing = llb.CacheMountLocked
				default:
					panic("unknown sharing mode")
				}

				opts = append(opts, llb.AsPersistentCacheDir(id, sharing))
			default:
				iopts, err := cg.EmitOptionExpr(ctx, scope, op, parser.NewIdentExpr(stmt.Call.Func.Ident.Name))
				if err != nil {
					return opts, stacktrace.Propagate(err, "")
				}
				opts = append(opts, iopts...)
			}
		}
	}
	return
}
