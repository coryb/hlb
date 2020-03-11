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
			parser.Filesystem: LookupByType{
				Func: map[string]FuncLookup{
					"scratch": FuncLookup{
						IsSource: true,
						Params:   []*parser.Field{},
					},
					"image": FuncLookup{
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "ref", false),
						},
					},
					"http": FuncLookup{
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "url", false),
						},
					},
					"git": FuncLookup{
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "remote", false),
							parser.NewField(parser.Str, "ref", false),
						},
					},
					"local": FuncLookup{
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"generate": FuncLookup{
						IsSource: true,
						Params: []*parser.Field{
							parser.NewField(parser.Filesystem, "frontend", false),
						},
					},
					"shell": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "arg", true),
						},
					},
					"run": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "arg", true),
						},
					},
					"env": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "key", false),
							parser.NewField(parser.Str, "value", false),
						},
					},
					"dir": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"user": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "name", false),
						},
					},
					"mkdir": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
							parser.NewField(parser.Int, "filemode", false),
						},
					},
					"mkfile": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
							parser.NewField(parser.Int, "filemode", false),
							parser.NewField(parser.Str, "content", false),
						},
					},
					"rm": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"copy": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Filesystem, "input", false),
							parser.NewField(parser.Str, "src", false),
							parser.NewField(parser.Str, "dst", false),
						},
					},
				},
			},
			"option::copy": LookupByType{
				Func: map[string]FuncLookup{
					"followSymlinks": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"contentsOnly": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"unpack": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"createDestPath": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
				},
			},
			"option::generate": LookupByType{
				Func: map[string]FuncLookup{
					"frontendInput": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "key", false),
							parser.NewField(parser.Filesystem, "value", false),
						},
					},
					"frontendOpt": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "key", false),
							parser.NewField(parser.Str, "value", false),
						},
					},
				},
			},
			"option::git": LookupByType{
				Func: map[string]FuncLookup{
					"keepGitDir": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
				},
			},
			"option::http": LookupByType{
				Func: map[string]FuncLookup{
					"checksum": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "digest", false),
						},
					},
					"chmod": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "filemode", false),
						},
					},
					"filename": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "name", false),
						},
					},
				},
			},
			"option::image": LookupByType{
				Func: map[string]FuncLookup{
					"resolve": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
				},
			},
			"option::local": LookupByType{
				Func: map[string]FuncLookup{
					"includePatterns": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "pattern", true),
						},
					},
					"excludePatterns": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "pattern", true),
						},
					},
					"followPaths": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", true),
						},
					},
				},
			},
			"option::mkdir": LookupByType{
				Func: map[string]FuncLookup{
					"createParents": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"chown": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "owner", false),
						},
					},
					"createdTime": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "created", false),
						},
					},
				},
			},
			"option::mkfile": LookupByType{
				Func: map[string]FuncLookup{
					"chown": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "owner", false),
						},
					},
					"createdTime": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "created", false),
						},
					},
				},
			},
			"option::mount": LookupByType{
				Func: map[string]FuncLookup{
					"readonly": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"tmpfs": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"sourcePath": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"cache": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "cacheid", false),
							parser.NewField(parser.Str, "sharingmode", false),
						},
					},
				},
			},
			"option::rm": LookupByType{
				Func: map[string]FuncLookup{
					"allowNotFound": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"allowWildcards": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
				},
			},
			"option::run": LookupByType{
				Func: map[string]FuncLookup{
					"readonlyRootfs": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"env": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "key", false),
							parser.NewField(parser.Str, "value", false),
						},
					},
					"dir": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"user": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "name", false),
						},
					},
					"network": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "networkmode", false),
						},
					},
					"security": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "securitymode", false),
						},
					},
					"host": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "hostname", false),
							parser.NewField(parser.Str, "address", false),
						},
					},
					"ssh": FuncLookup{
						IsSource: false,
						Params:   []*parser.Field{},
					},
					"secret": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "localPath", false),
							parser.NewField(parser.Str, "mountPoint", false),
						},
					},
					"mount": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Filesystem, "input", false),
							parser.NewField(parser.Str, "mountPoint", false),
						},
					},
				},
			},
			"option::secret": LookupByType{
				Func: map[string]FuncLookup{
					"uid": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "id", false),
						},
					},
					"gid": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "id", false),
						},
					},
					"mode": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "filemode", false),
						},
					},
				},
			},
			"option::ssh": LookupByType{
				Func: map[string]FuncLookup{
					"target": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "mountPoint", false),
						},
					},
					"localPath": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Str, "path", false),
						},
					},
					"uid": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "id", false),
						},
					},
					"gid": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "id", false),
						},
					},
					"mode": FuncLookup{
						IsSource: false,
						Params: []*parser.Field{
							parser.NewField(parser.Int, "filemode", false),
						},
					},
				},
			},
			parser.Str: LookupByType{
				Func: map[string]FuncLookup{
					"format": FuncLookup{
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
