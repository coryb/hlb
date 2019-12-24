package hlb

import (
	"bytes"
	"io"

	"github.com/alecthomas/participle"
	"github.com/alecthomas/participle/lexer"
)

var (
	parser = participle.MustBuild(&AST{})
)

func Parse(r io.Reader) (*AST, error) {
	ast := &AST{}

	name := lexer.NameOfReader(r)
	if name == "" {
		name = "<stdin>"
	}

	ib := &indexedBuffer{buf: new(bytes.Buffer)}
	r = io.TeeReader(r, ib)

	lex, err := lexer.TextScannerLexer.Lex(&namedReader{r, name})
	if err != nil {
		return nil, err
	}

	peeker, err := lexer.Upgrade(lex)
	if err != nil {
		return nil, err
	}

	err = parser.ParseFromLexer(peeker, ast)
	if err != nil {
		perr, ok := err.(participle.Error)
		if !ok {
			return ast, err
		}

		nerr, err := NewError(ib, peeker, perr)
		if err != nil {
			return ast, err
		}

		return ast, nerr

		// fmt.Println(perr.Position().Filename)

		// pos := perr.Position()

		// terr, ok := perr.(participle.UnexpectedTokenError)
		// if !ok {
		// 	return ast, perr
		// }

		// fmt.Printf("%d | %s [%d]", pos.Line, line, pos.Offset)
		// fmt.Printf("     %s%s\n", strings.Repeat(" ", pos.Column), strings.Repeat("^", len(terr.Token.Value)))
		// return ast, perr
	}

	return ast, nil
}

type AST struct {
	Pos     lexer.Position
	Entries []*Entry `( @@ ( ";" )?)*`
}

type Entry struct {
	Pos   lexer.Position
	State *NamedState `"state"  @@`
	// Option *NamedOption `| "option" @@`
	// Result *NamedResult `| "result" @@`
	// Frontend *NamedFrontend `| "frontend" @@`
}

type NamedState struct {
	Pos  lexer.Position
	Name string     `@Ident`
	Body *StateBody `@@`
}

type State struct {
	Pos  lexer.Position
	Body *StateBody `( ( "state" )? @@`
	Name *string    `| @Ident )`
}

type StateBody struct {
	Pos      lexer.Position
	Source   Source   `"{" @@ ( ";" )?`
	Ops      []*Op    `( @@ ( ";" )? )*`
	BlockEnd BlockEnd `@@`
}

type Source struct {
	Pos     lexer.Position
	Image   *Image  `( "image" @@`
	HTTP    *HTTP   `| "http" @@`
	Git     *Git    `| "git" @@`
	Scratch *string `| @("scratch")? )`
}

type Image struct {
	Pos    lexer.Position
	Ref    Literal      `@@`
	Option *ImageOption `( "with" @@ )?`
}

type ImageOption struct {
	Pos         lexer.Position
	ImageFields []*ImageField `( "option" "{" ( @@ [ ";" ] )*`
	BlockEnd    BlockEnd      `@@`
	Name        *string       `| @Ident )`
}

type ImageField struct {
	Pos     lexer.Position
	Resolve *bool `@("resolve")`
}

type HTTP struct {
	Pos lexer.Position
	URL Literal `@@`
}

type Git struct {
	Pos    lexer.Position
	Remote Literal `@@`
	Ref    Literal `@@`
}

type Op struct {
	Pos    lexer.Position
	Exec   *Exec   `( "exec" @@`
	Env    *Env    `| "env" @@`
	Dir    *Dir    `| "dir" @@`
	User   *User   `| "user" @@`
	Mkdir  *Mkdir  `| "mkdir" @@`
	Mkfile *Mkfile `| "mkfile" @@`
	Rm     *Rm     `| "rm" @@`
	Copy   *Copy   `| "copy" @@ )`
}

type Exec struct {
	Pos   lexer.Position
	Shlex Literal `@@`
}

type Env struct {
	Pos   lexer.Position
	Key   Literal `@@`
	Value Literal `@@`
}

type Literal struct {
	Pos lexer.Position
	Str *string `@(String|Char|RawString)`
}

func (l Literal) String() string {
	return *l.Str
}

type Dir struct {
	Pos  lexer.Position
	Path Literal `@@`
}

type User struct {
	Pos  lexer.Position
	Name Literal `@@`
}

type Copy struct {
	Pos  lexer.Position
	From State   `@@`
	Src  Literal `@@`
	Dst  Literal `@@`
}

type Mkdir struct {
	Pos  lexer.Position
	Path Literal  `@@`
	Mode FileMode `@@`
}

type Mkfile struct {
	Pos     lexer.Position
	Path    Literal  `@@`
	Mode    FileMode `@@`
	Content Literal  `( @@ )?`
}

type FileMode struct {
	Pos   lexer.Position
	Value uint32 `@Int`
}

type Rm struct {
	Pos  lexer.Position
	Path Literal `@@`
}

type NamedResult struct {
	Pos lexer.Position
}

type NamedOption struct {
	Pos lexer.Position
}

type BlockEnd struct {
	Pos   lexer.Position
	Brace string `@("}")`
}
