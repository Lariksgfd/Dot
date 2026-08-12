// Package ast provides a printer that renders an AST as a deterministic,
// indented tree. String scalar values are rendered bare (not quoted) so the
// output stays readable; boolean flags appear as `key=true` only when true;
// nil children render as `<nil>`. Positions are never printed, keeping the
// output stable for golden tests (D24).
package ast

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dotlang/dot/lexer"
)

// Print returns an indented tree dump of n. Positions are never printed.
func Print(n Node) string {
	var sb strings.Builder
	Fprint(&sb, n)
	return sb.String()
}

// Fprint writes Print(n) to w. It returns any write error encountered.
func Fprint(w io.Writer, n Node) error {
	p := &printer{w: w}
	p.print(n)
	return p.err
}

// printer writes an indented AST tree to an io.Writer.
type printer struct {
	w      io.Writer
	indent int
	err    error
	comment []lexer.Token
}

// line writes the indentation followed by the formatted text and a newline.
func (p *printer) line(format string, args ...any) {
	if p.err != nil {
		return
	}
	for i := 0; i < p.indent; i++ {
		io.WriteString(p.w, "  ")
	}
	fmt.Fprintf(p.w, format, args...)
	if len(p.comment) > 0 {
		for _, c := range p.comment {
			fmt.Fprintf(p.w, " %s", c.Lexeme)
		}
		p.comment = nil
	}
	io.WriteString(p.w, "\n")
}

// leaf prints a node with attributes and no children.
func (p *printer) leaf(name, attrs string) {
	if attrs != "" {
		p.line("%s %s", name, attrs)
	} else {
		p.line("%s", name)
	}
}

// nodeHead opens a node: prints its name+attrs and increases the indent.
func (p *printer) nodeHead(name, attrs string) {
	p.leaf(name, attrs)
	p.indent++
}

// nodeTail closes a node by decreasing the indent.
func (p *printer) nodeTail() {
	p.indent--
}

// child prints a child node at the deeper indent, or `<nil>` if n is nil.
func (p *printer) child(n Node) {
	if isNilNode(n) {
		p.line("<nil>")
		return
	}
	p.print(n)
}

// walkFunc is a generic helper that walks a slice of nodes, calling p.child
// for each element.
func walkFunc[T Node](p *printer, list []T) {
	for i := range list {
		p.child(list[i])
	}
}

// joinAttrs joins non-empty attribute strings with spaces.
func joinAttrs(parts ...string) string {
	n := 0
	for _, p := range parts {
		if p != "" {
			parts[n] = p
			n++
		}
	}
	return strings.Join(parts[:n], " ")
}

// attr returns `key=value` when value is non-empty, otherwise "".
func attr(key, value string) string {
	if value == "" {
		return ""
	}
	return key + "=" + value
}

// attrBool returns `key=true` when flag is true, otherwise "".
func attrBool(key string, flag bool) string {
	if !flag {
		return ""
	}
	return key + "=true"
}

// attrU64 returns `key=value` for a non-zero uint64, otherwise "".
func attrU64(key string, value uint64) string {
	if value == 0 {
		return ""
	}
	return key + "=" + strconv.FormatUint(value, 10)
}

// attrF64 returns `key=value` for a non-zero float64, otherwise "".
func attrF64(key string, value float64) string {
	if value == 0 {
		return ""
	}
	return key + "=" + strconv.FormatFloat(value, 'g', -1, 64)
}

// attrBase returns `key=value` for a non-zero int (used for numeric base).
func attrBase(key string, value int) string {
	if value == 0 {
		return ""
	}
	return key + "=" + strconv.Itoa(value)
}

// print is the big type switch over statement, declaration, and helper
// nodes. Anything it does not handle is delegated to printExprTypePattern.
func (p *printer) print(n Node) {
	if doc := n.GetDoc(); len(doc) > 0 {
		for _, d := range doc {
			p.line("%s", d.Lexeme)
		}
	}
	if comm := n.GetComment(); len(comm) > 0 {
		p.comment = comm
	}
	switch v := n.(type) {
	case *Program:
		p.nodeHead("Program", attr("file", v.File))
		for i := range v.Decls {
			p.child(v.Decls[i])
		}
		p.nodeTail()
	case *BlockStmt:
		p.nodeHead("BlockStmt", "")
		walkFunc(p, v.Stmts)
		p.nodeTail()
	case *ExprStmt:
		p.nodeHead("ExprStmt", "")
		p.child(v.X)
		p.nodeTail()
	case *ReturnStmt:
		p.nodeHead("ReturnStmt", "")
		walkFunc(p, v.Values)
		p.nodeTail()
	case *BreakStmt:
		p.leaf("BreakStmt", attr("label", v.Label))
	case *ContinueStmt:
		p.leaf("ContinueStmt", attr("label", v.Label))
	case *DeferStmt:
		p.nodeHead("DeferStmt", "")
		p.child(v.Call)
		p.nodeTail()
	case *PerfBlock:
		p.nodeHead("PerfBlock", "")
		p.child(v.Block)
		p.nodeTail()
	case *BadStmt:
		p.leaf("BadStmt", "")
	case *VarDecl:
		p.nodeHead("VarDecl", varDeclAttrs(v))
		walkFunc(p, v.Annotations)
		walkFunc(p, v.Names)
		p.child(v.Type)
		walkFunc(p, v.Values)
		p.nodeTail()
	case *ForStmt:
		p.nodeHead("ForStmt", forStmtAttrs(v))
		p.child(v.Key)
		p.child(v.Value)
		p.child(v.Iterable)
		p.child(v.Cond)
		p.child(v.Body)
		p.nodeTail()
	case *FnDecl:
		p.nodeHead("FnDecl", fnDeclAttrs(v))
		walkFunc(p, v.Annotations)
		walkFunc(p, v.TypeParams)
		p.child(v.Sig)
		p.child(v.Body)
		p.child(v.ExprBody)
		p.nodeTail()
	case *StructDecl:
		p.nodeHead("StructDecl", namedAttrs(v.Name, v.Pub))
		walkFunc(p, v.Annotations)
		walkFunc(p, v.TypeParams)
		for i := range v.Fields {
			p.child(v.Fields[i])
		}
		p.nodeTail()
	case *EnumDecl:
		p.nodeHead("EnumDecl", namedAttrs(v.Name, v.Pub))
		walkFunc(p, v.Annotations)
		walkFunc(p, v.TypeParams)
		for i := range v.Variants {
			p.child(v.Variants[i])
		}
		p.nodeTail()
	case *TraitDecl:
		p.nodeHead("TraitDecl", namedAttrs(v.Name, v.Pub))
		walkFunc(p, v.Annotations)
		walkFunc(p, v.TypeParams)
		for i := range v.Methods {
			p.child(v.Methods[i])
		}
		p.nodeTail()
	case *ImplDecl:
		p.nodeHead("ImplDecl", "")
		walkFunc(p, v.Annotations)
		walkFunc(p, v.TypeParams)
		p.child(v.Trait)
		p.child(v.Type)
		for i := range v.Methods {
			p.child(v.Methods[i])
		}
		p.nodeTail()
	case *ImportDecl:
		p.nodeHead("ImportDecl", importDeclAttrs(v))
		p.nodeTail()
	case *MatchArm:
		p.nodeHead("MatchArm", "")
		p.child(v.Pattern)
		p.child(v.Body)
		p.nodeTail()
	case *FnSig:
		p.nodeHead("FnSig", "")
		p.child(v.Recv)
		walkFunc(p, v.Params)
		p.child(v.Result)
		p.nodeTail()
	case *Param:
		p.nodeHead("Param", paramAttrs(v))
		p.child(v.Type)
		p.child(v.Default)
		p.nodeTail()
	case *FieldDecl:
		p.nodeHead("FieldDecl", fieldDeclAttrs(v))
		walkFunc(p, v.Annotations)
		p.child(v.Type)
		p.child(v.Default)
		p.nodeTail()
	case *EnumVariant:
		p.nodeHead("EnumVariant", enumVariantAttrs(v))
		walkFunc(p, v.Fields)
		p.nodeTail()
	case *Annotation:
		p.nodeHead("Annotation", attr("name", v.Name))
		walkFunc(p, v.Args)
		p.nodeTail()
	case *TypeParam:
		p.nodeHead("TypeParam", attr("name", v.Name))
		walkFunc(p, v.Bounds)
		p.nodeTail()
	default:
		p.printExprTypePattern(n)
	}
}

// varDeclAttrs returns the attribute string for a VarDecl.
func varDeclAttrs(v *VarDecl) string {
	return joinAttrs(attrBool("const", v.Const), attrBool("pub", v.Pub))
}

// forStmtAttrs returns the attribute string for a ForStmt.
func forStmtAttrs(v *ForStmt) string {
	return joinAttrs(attr("kind", v.Kind.String()), attr("label", v.Label))
}

// fnDeclAttrs returns the attribute string for a FnDecl.
func fnDeclAttrs(v *FnDecl) string {
	return joinAttrs(
		attr("name", v.Name),
		attrBool("pub", v.Pub),
		attrBool("async", v.Async),
	)
}

// namedAttrs returns `name=X [pub=true]` for a named declaration.
func namedAttrs(name string, pub bool) string {
	return joinAttrs(attr("name", name), attrBool("pub", pub))
}

// importDeclAttrs returns the attribute string for an ImportDecl.
func importDeclAttrs(v *ImportDecl) string {
	return joinAttrs(
		attr("path", strings.Join(v.Path, ".")),
		attrBool("from", v.From),
		attr("alias", v.Alias),
	)
}

// paramAttrs returns the attribute string for a Param.
func paramAttrs(v *Param) string {
	return joinAttrs(
		attr("name", v.Name),
		attrBool("mut", v.Mut),
		attrBool("variadic", v.Variadic),
		attrBool("self", v.IsSelf),
	)
}

// fieldDeclAttrs returns the attribute string for a FieldDecl.
func fieldDeclAttrs(v *FieldDecl) string {
	return joinAttrs(attrBool("embed", v.Embed), attr("name", v.Name))
}

// enumVariantAttrs returns the attribute string for an EnumVariant.
func enumVariantAttrs(v *EnumVariant) string {
	return joinAttrs(attr("name", v.Name), attrBool("parens", v.HasParens))
}
