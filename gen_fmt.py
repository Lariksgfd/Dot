import os
import re

ast_files = ["decl.go", "expr.go", "expr_lit.go", "stmt.go", "type.go", "pattern.go"]
structs = {}

for f in ast_files:
    if not os.path.exists(f"ast/{f}"): continue
    with open(f"ast/{f}", "r", encoding="utf-8") as file:
        content = file.read()
        
    matches = re.finditer(r"type (\w+) struct \{([\s\S]*?)\n\}", content)
    for m in matches:
        name = m.group(1)
        fields_text = m.group(2)
        fields = []
        for line in fields_text.split("\n"):
            line = line.strip()
            if not line or line.startswith("//"): continue
            if "//" in line: line = line.split("//")[0].strip()
            parts = line.split()
            if len(parts) >= 2:
                fields.append((parts[0], parts[1]))
        structs[name] = fields

out = """package ast

import (
	"strings"
	"fmt"
)

// Format prints the AST back into source code.
func Format(n Node) string {
	var sb strings.Builder
	p := &formatter{w: &sb}
	p.formatNode(n)
	return sb.String()
}

type formatter struct {
	w       *strings.Builder
	indent  int
	lineBegan bool
}

func (p *formatter) write(s string) {
	if !p.lineBegan {
		p.w.WriteString(strings.Repeat("    ", p.indent))
		p.lineBegan = true
	}
	p.w.WriteString(s)
}

func (p *formatter) writeLn(s string) {
	p.write(s)
	p.w.WriteString("\\n")
	p.lineBegan = false
}

func (p *formatter) formatNode(n Node) {
	if isNilNode(n) {
		return
	}

	// Docs
	if docs := n.GetDoc(); len(docs) > 0 {
		for _, doc := range docs {
			p.writeLn(doc.Lexeme)
		}
	}

	switch n := n.(type) {
"""

# Structs that are not Nodes:
not_nodes = ["ImportName", "Arg", "StringPart", "TupleElem", "MapEntry", "StructLitField", "StructPatternField"]

def generate_case(name, fields):
    if name in not_nodes: return ""
    code = f'\tcase *{name}:\n'
    
    if name == "Program":
        code += '\t\tfor i, decl := range n.Decls {\n'
        code += '\t\t\tif i > 0 { p.writeLn("") }\n'
        code += '\t\t\tp.formatNode(decl)\n'
        code += '\t\t\tp.writeLn("")\n'
        code += '\t\t}\n'
        code += '\t\tfor _, comment := range n.LooseComments {\n'
        code += '\t\t\tp.writeLn(comment.Lexeme)\n'
        code += '\t\t}\n'
        return code
    
    if name == "BlockStmt":
        code += '\t\tp.write("{")\n'
        code += '\t\tp.writeLn("")\n'
        code += '\t\tp.indent++\n'
        code += '\t\tfor _, stmt := range n.Stmts {\n'
        code += '\t\t\tp.formatNode(stmt)\n'
        code += '\t\t\tp.writeLn("")\n'
        code += '\t\t}\n'
        code += '\t\tp.indent--\n'
        code += '\t\tp.write("}")\n'
        return code
        
    if name == "BlockExpr":
        code += '\t\tp.write("{")\n'
        code += '\t\tp.writeLn("")\n'
        code += '\t\tp.indent++\n'
        code += '\t\tfor _, stmt := range n.Block.Stmts {\n'
        code += '\t\t\tp.formatNode(stmt)\n'
        code += '\t\t\tp.writeLn("")\n'
        code += '\t\t}\n'
        code += '\t\tp.indent--\n'
        code += '\t\tp.write("}")\n'
        return code
    
    code += f'\t\t// p.write("/* {name} */")\n'

    for fname, ftype in fields:
        if fname in ["BaseNode", "NamePos", "Doc", "Comment", "KwPos", "ForPos", "LParen", "RParen", "ArrowPos", "AtPos", "ColonPos", "LBrace", "RBrace"]: continue
        
        if fname == "Pub":
            code += '\t\tif n.Pub { p.write("pub ") }\n'
            continue
        if fname == "Async":
            code += '\t\tif n.Async { p.write("async ") }\n'
            continue
            
        if fname == "Name" and ftype == "string":
            code += '\t\tp.write(n.Name)\n'
            continue
            
        if ftype.startswith("*") or ftype in ["Expr", "Type", "Stmt", "Decl", "Pattern", "Node"]:
            code += f'\t\tp.formatNode(n.{fname})\n'
        elif ftype.startswith("[]") and (ftype.endswith("Expr") or ftype.endswith("Type") or ftype.endswith("Stmt") or ftype.endswith("Decl") or ftype.endswith("Pattern") or ftype.startswith("[]*")):
            code += f'\t\tfor i, c := range n.{fname} {{\n'
            code += f'\t\t\tif i > 0 {{ p.write(", ") }}\n'
            code += f'\t\t\tp.formatNode(c)\n'
            code += f'\t\t}}\n'
        elif ftype == "string":
            code += f'\t\tp.write(n.{fname})\n'
            
    return code

for name, fields in structs.items():
    if name == "BaseNode": continue
    out += generate_case(name, fields)

out += """
	default:
		p.write(fmt.Sprintf("/* UNIMPLEMENTED: %T */", n))
	}

	// Comments
	if comments := n.GetComment(); len(comments) > 0 {
		for _, comment := range comments {
			p.write(" ")
			p.write(comment.Lexeme)
		}
	}
}
"""

with open("ast/format.go", "w", encoding="utf-8") as f:
    f.write(out)
