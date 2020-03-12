// Code generated by builtingen ../language/builtin.hlb ../builtin/builtin.go; DO NOT EDIT.

package builtin

import "github.com/openllb/hlb/parser"

type BuiltinLookup struct {
	ByType map[parser.ObjType]LookupByType
}

type LookupByType struct {
	Func map[string]FuncLookup
}

type FuncLookup struct {
	IsSource bool
	Params   []*parser.Field
}

var (
	Lookup = BuiltinLookup{
		ByType: map[parser.ObjType]LookupByType{
			parser.Filesystem: {
				Func: map[string]FuncLookup{
					"scratch": {
						IsSource: true,
						Params:   []*parser.Field{},
					},
					"image": {
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "ref", false),
						},
					},
					"http": {
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "url", false),
						},
					},
					"git": {
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "remote", false),
							parser.NewField(parser.Str, "ref", false),
						},
					},
					"local": {
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"generate": {
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Filesystem, "frontend", false),
						},
					},
					"shell": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "arg", true),
						},
					},
					"run": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "arg", true),
						},
					},
					"env": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "key", false),
							parser.NewField(parser.Str, "value", false),
						},
					},
					"dir": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"user": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "name", false),
						},
					},
					"mkdir": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
							parser.NewField(parser.Int, "filemode", false),
						},
					},
					"mkfile": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
							parser.NewField(parser.Int, "filemode", false),
							parser.NewField(parser.Str, "content", false),
						},
					},
					"rm": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"copy": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Filesystem, "input", false),
							parser.NewField(parser.Str, "src", false),
							parser.NewField(parser.Str, "dst", false),
						},
					},
				},
			},
			"option::copy": {
				Func: map[string]FuncLookup{
					"followSymlinks": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"contentsOnly": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"unpack": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"createDestPath": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
				},
			},
			"option::generate": {
				Func: map[string]FuncLookup{
					"frontendInput": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "key", false),
							parser.NewField(parser.Filesystem, "value", false),
						},
					},
					"frontendOpt": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "key", false),
							parser.NewField(parser.Str, "value", false),
						},
					},
				},
			},
			"option::git": {
				Func: map[string]FuncLookup{
					"keepGitDir": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
				},
			},
			"option::http": {
				Func: map[string]FuncLookup{
					"checksum": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "digest", false),
						},
					},
					"chmod": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "filemode", false),
						},
					},
					"filename": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "name", false),
						},
					},
				},
			},
			"option::image": {
				Func: map[string]FuncLookup{
					"resolve": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
				},
			},
			"option::local": {
				Func: map[string]FuncLookup{
					"includePatterns": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "pattern", true),
						},
					},
					"excludePatterns": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "pattern", true),
						},
					},
					"followPaths": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", true),
						},
					},
				},
			},
			"option::mkdir": {
				Func: map[string]FuncLookup{
					"createParents": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"chown": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "owner", false),
						},
					},
					"createdTime": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "created", false),
						},
					},
				},
			},
			"option::mkfile": {
				Func: map[string]FuncLookup{
					"chown": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "owner", false),
						},
					},
					"createdTime": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "created", false),
						},
					},
				},
			},
			"option::mount": {
				Func: map[string]FuncLookup{
					"readonly": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"tmpfs": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"sourcePath": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"cache": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "cacheid", false),
							parser.NewField(parser.Str, "sharingmode", false),
						},
					},
				},
			},
			"option::rm": {
				Func: map[string]FuncLookup{
					"allowNotFound": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"allowWildcards": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
				},
			},
			"option::run": {
				Func: map[string]FuncLookup{
					"readonlyRootfs": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"env": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "key", false),
							parser.NewField(parser.Str, "value", false),
						},
					},
					"dir": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"user": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "name", false),
						},
					},
					"network": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "networkmode", false),
						},
					},
					"security": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "securitymode", false),
						},
					},
					"host": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "hostname", false),
							parser.NewField(parser.Str, "address", false),
						},
					},
					"ssh": {
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"secret": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "localPath", false),
							parser.NewField(parser.Str, "mountPoint", false),
						},
					},
					"mount": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Filesystem, "input", false),
							parser.NewField(parser.Str, "mountPoint", false),
						},
					},
				},
			},
			"option::secret": {
				Func: map[string]FuncLookup{
					"uid": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "id", false),
						},
					},
					"gid": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "id", false),
						},
					},
					"mode": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "filemode", false),
						},
					},
				},
			},
			"option::ssh": {
				Func: map[string]FuncLookup{
					"target": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "mountPoint", false),
						},
					},
					"localPath": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"uid": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "id", false),
						},
					},
					"gid": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "id", false),
						},
					},
					"mode": {
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "filemode", false),
						},
					},
				},
			},
			parser.Str: {
				Func: map[string]FuncLookup{
					"format": {
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "formatString", false),
							parser.NewField(parser.Str, "values", true),
						},
					},
				},
			},
		},
	}
)
