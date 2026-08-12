package llvm

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
	"github.com/dotlang/dot/types"
)

type emitter struct {
	info         *types.Info
	prog         *ast.Program
	sb           strings.Builder
	tmpID        int
	lblID        int
	structFields map[string]map[string]int
	enumVariants map[string]map[string]int
	ptrVars      map[string]bool
	varTypes     map[string]string
	stringLits   map[string]string
	implMethods  map[string]*ast.FnDecl
}

func (e *emitter) nextTmp() string {
	e.tmpID++
	return fmt.Sprintf("%%t%d", e.tmpID)
}

func (e *emitter) nextLabel(prefix string) string {
	e.lblID++
	return fmt.Sprintf("%s%d", prefix, e.lblID)
}

func (e *emitter) emit(format string, args ...any) {
	e.sb.WriteString(fmt.Sprintf(format, args...))
	e.sb.WriteString("\n")
}

func Generate(info *types.Info, prog *ast.Program) (string, error) {
	e := &emitter{
		info:         info,
		prog:         prog,
		structFields: make(map[string]map[string]int),
		enumVariants: make(map[string]map[string]int),
		ptrVars:      make(map[string]bool),
		varTypes:     make(map[string]string),
		stringLits:   make(map[string]string),
	}

	e.emit("declare void @dot_retain(ptr)")
	e.emit("declare void @dot_release(ptr)")
	e.emit("declare i32 @puts(ptr)")
	e.emit("")

	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *ast.StructDecl:
			e.emitStructDecl(d)
		case *ast.EnumDecl:
			e.emitEnumDecl(d)
		}
	}

	for _, decl := range prog.Decls {
		if fn, ok := decl.(*ast.FnDecl); ok {
			if len(fn.TypeParams) == 0 {
				e.emitFn(fn, fn.Name)
			}
		}
	}

	if e.info != nil {
		for _, inst := range e.info.InstanceList {
			if inst.Generic != nil && inst.Generic.Kind == types.SymFunc {
				if fn, ok := inst.Generic.Decl.(*ast.FnDecl); ok {
					e.emitFn(fn, inst.Mangled)
				}
			}
		}
	}

	var globalsSb strings.Builder
	globalsSb.WriteString("; ModuleID = 'main'\n")
	globalsSb.WriteString(fmt.Sprintf("source_filename = \"%s\"\n\n", prog.File))
	
	for content, name := range e.stringLits {
		length := len(content) + 1
		globalsSb.WriteString(fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\\00\"\n", name, length, content))
	}
	globalsSb.WriteString("\n")

	return globalsSb.String() + e.sb.String(), nil
}

func (e *emitter) emitStructDecl(sd *ast.StructDecl) {
	var fields []string
	fieldMap := make(map[string]int)
	for i, f := range sd.Fields {
		fields = append(fields, "i32") // Defaulting to i32 for now
		fieldMap[f.Name] = i
	}
	e.structFields[sd.Name] = fieldMap
	e.emit("%%%s = type { %s }", sd.Name, strings.Join(fields, ", "))
}

func (e *emitter) emitEnumDecl(ed *ast.EnumDecl) {
	maxFields := 0
	for _, v := range ed.Variants {
		if len(v.Fields) > maxFields {
			maxFields = len(v.Fields)
		}
	}

	variantMap := make(map[string]int)
	for i, v := range ed.Variants {
		variantMap[v.Name] = i
	}
	e.enumVariants[ed.Name] = variantMap

	fields := make([]string, 0, 1+maxFields)
	fields = append(fields, "i32")
	for i := 0; i < maxFields; i++ {
		fields = append(fields, "i32")
	}
	e.emit("%%%s = type { %s }", ed.Name, strings.Join(fields, ", "))
}

func (e *emitter) emitEnumLit(enumName string, variantName string, args []ast.Expr) string {
	tmp := e.nextTmp()
	e.emit("  %s = alloca %%%s", tmp, enumName)

	tag := 0
	if vmap, ok := e.enumVariants[enumName]; ok {
		tag = vmap[variantName]
	}

	tagPtr := e.nextTmp()
	e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 0", tagPtr, enumName, tmp)
	e.emit("  store i32 %d, ptr %s", tag, tagPtr)

	for i, arg := range args {
		val := e.emitExpr(arg)
		fieldPtr := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, enumName, tmp, i+1)
		e.emit("  store i32 %s, ptr %s", val, fieldPtr)
	}

	return tmp
}

func (e *emitter) matchEnumNameFromPatterns(me *ast.MatchExpr) string {
	for _, arm := range me.Arms {
		pat := arm.Pattern
		if gp, ok := pat.(*ast.GuardedPattern); ok {
			pat = gp.Pattern
		}
		if ep, ok := pat.(*ast.EnumPattern); ok {
			if ep.Enum != "" {
				return ep.Enum
			}
		}
	}
	return ""
}

func (e *emitter) emitMatchExpr(me *ast.MatchExpr) string {
	enumName := e.matchEnumNameFromPatterns(me)

	subjectPtr := e.emitExpr(me.Subject)

	resPtr := "%" + e.nextLabel("match.res")
	e.emit("  %s = alloca i32", resPtr)

	endBlock := e.nextLabel("match.end")
	failBlock := e.nextLabel("match.default")

	var armLabels []string
	for i := range me.Arms {
		armLabels = append(armLabels, e.nextLabel(fmt.Sprintf("match.arm%d", i)))
	}
	nextLabels := make([]string, len(me.Arms))
	for i := range me.Arms {
		nextLabels[i] = e.nextLabel(fmt.Sprintf("match.next%d", i))
	}

	var tagVal string
	if enumName != "" {
		tagPtr := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 0", tagPtr, enumName, subjectPtr)
		tagVal = e.nextTmp()
		e.emit("  %s = load i32, ptr %s", tagVal, tagPtr)
	} else {
		tagVal = subjectPtr
	}

	for i, arm := range me.Arms {
		if i == 0 {
			e.emit("  br label %%%s", armLabels[0])
		}

		e.emit("\n%s:", armLabels[i])
		pat := arm.Pattern
		var guardExpr ast.Expr
		if gp, ok := pat.(*ast.GuardedPattern); ok {
			pat = gp.Pattern
			guardExpr = gp.Guard
		}

		var branchCond string

		if ep, ok := pat.(*ast.EnumPattern); ok {
			tag := 0
			epEnumName := ep.Enum
			if epEnumName == "" {
				epEnumName = enumName
			}
			if vmap, ok := e.enumVariants[epEnumName]; ok {
				tag = vmap[ep.Variant]
			}
			cmpTmp := e.nextTmp()
			e.emit("  %s = icmp eq i32 %s, %d", cmpTmp, tagVal, tag)
			branchCond = cmpTmp
		} else if lp, ok := pat.(*ast.LiteralPattern); ok {
			litVal := e.emitExpr(lp.Value)
			cmpTmp := e.nextTmp()
			e.emit("  %s = icmp eq i32 %s, %s", cmpTmp, tagVal, litVal)
			branchCond = cmpTmp
		} else if _, ok := pat.(*ast.WildcardPattern); ok {
			bodyVal := e.emitExpr(arm.Body)
			e.emit("  store i32 %s, ptr %s", bodyVal, resPtr)
			e.emit("  br label %%%s", endBlock)
			continue
		} else if ip, ok := pat.(*ast.IdentPattern); ok {
			e.emit("  %%%s = alloca i32", ip.Name)
			e.emit("  store i32 %s, ptr %%%s", tagVal, ip.Name)
			if guardExpr != nil {
				guardVal := e.emitExpr(guardExpr)
				branchCond = guardVal
				if i+1 < len(armLabels) {
					e.emit("  br i1 %s, label %%%s, label %%%s", branchCond, nextLabels[i], armLabels[i+1])
				} else {
					e.emit("  br i1 %s, label %%%s, label %%%s", branchCond, nextLabels[i], failBlock)
				}
				e.emit("\n%s:", nextLabels[i])
			}
			bodyVal := e.emitExpr(arm.Body)
			e.emit("  store i32 %s, ptr %s", bodyVal, resPtr)
			e.emit("  br label %%%s", endBlock)
			continue
		} else {
			bodyVal := e.emitExpr(arm.Body)
			e.emit("  store i32 %s, ptr %s", bodyVal, resPtr)
			e.emit("  br label %%%s", endBlock)
			continue
		}

		if guardExpr != nil {
			guardVal := e.emitExpr(guardExpr)
			andTmp := e.nextTmp()
			e.emit("  %s = and i1 %s, %s", andTmp, branchCond, guardVal)
			branchCond = andTmp
		}

		if i+1 < len(armLabels) {
			e.emit("  br i1 %s, label %%%s, label %%%s", branchCond, nextLabels[i], armLabels[i+1])
		} else {
			e.emit("  br i1 %s, label %%%s, label %%%s", branchCond, nextLabels[i], failBlock)
		}

		e.emit("\n%s:", nextLabels[i])
		bodyVal := e.emitExpr(arm.Body)
		e.emit("  store i32 %s, ptr %s", bodyVal, resPtr)
		e.emit("  br label %%%s", endBlock)
	}

	e.emit("\n%s:", failBlock)
	e.emit("  store i32 0, ptr %s", resPtr)
	e.emit("  br label %%%s", endBlock)

	e.emit("\n%s:", endBlock)
	tmp := e.nextTmp()
	e.emit("  %s = load i32, ptr %s", tmp, resPtr)
	return tmp
}

func (e *emitter) emitFn(fn *ast.FnDecl, name string) {
	var params []string
	if fn.Sig != nil {
		for _, p := range fn.Sig.Params {
			params = append(params, fmt.Sprintf("i32 %%%s.arg", p.Name))
		}
	}
	e.emit("define i32 @%s(%s) {", name, strings.Join(params, ", "))
	e.emit("entry:")
	if fn.Sig != nil {
		for _, p := range fn.Sig.Params {
			e.emit("  %%%s = alloca i32", p.Name)
			e.emit("  store i32 %%%s.arg, ptr %%%s", p.Name, p.Name)
		}
	}
	if fn.Body != nil {
		for _, stmt := range fn.Body.Stmts {
			e.emitStmt(stmt)
		}
	}
	e.emit("  ret i32 0")
	e.emit("}")
}

func (e *emitter) isPtrValue(expr ast.Expr) bool {
	switch ex := expr.(type) {
	case *ast.UnaryExpr:
		return ex.Op == lexer.TokenAmp
	case *ast.Ident:
		return e.ptrVars[ex.Name]
	default:
		return false
	}
}

func (e *emitter) emitStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		e.emitExpr(s.X)
	case *ast.VarDecl:
		for i, nameExpr := range s.Names {
			if ident, ok := nameExpr.(*ast.Ident); ok {
				var val string = "0"
				isPtr := false
				var expr ast.Expr
				if i < len(s.Values) {
					expr = s.Values[i]
					isPtr = e.isPtrValue(expr)
					val = e.emitExpr(expr)
				}
				
				var declType types.Type
				if e.info != nil {
					declType = e.info.Types[ident]
				}
				needsARC := (declType != nil && types.IsHeap(declType)) || (expr != nil && e.info != nil && e.info.Escapes[expr] == types.Heap)

				isReassign := false
				if _, ok := e.varTypes[ident.Name]; ok {
					isReassign = true
				} else if e.ptrVars[ident.Name] {
					isReassign = true
				}

				if needsARC {
					e.emit("  call void @dot_retain(ptr %s)", val)
					if isReassign {
						oldVal := e.nextTmp()
						if e.ptrVars[ident.Name] {
							e.emit("  %s = load ptr, ptr %%%s", oldVal, ident.Name)
						} else {
							e.emit("  %s = load %s, ptr %%%s", oldVal, e.varTypes[ident.Name], ident.Name)
						}
						e.emit("  call void @dot_release(ptr %s)", oldVal)
					}
				}

				if isPtr {
					if !isReassign {
						e.ptrVars[ident.Name] = true
						e.emit("  %%%s = alloca ptr", ident.Name)
					}
					e.emit("  store ptr %s, ptr %%%s", val, ident.Name)
				} else {
					llvmType := "i32"
					if s.Type != nil {
						llvmType = e.resolveType(s.Type)
					} else if vt, ok := e.varTypes[ident.Name]; ok {
						llvmType = vt
					}
					if !isReassign {
						e.varTypes[ident.Name] = llvmType
						e.emit("  %%%s = alloca %s", ident.Name, llvmType)
					}
					e.emit("  store %s %s, ptr %%%s", llvmType, val, ident.Name)
				}
			}
		}
	case *ast.ForStmt:
		switch s.Kind {
		case ast.ForIn:
			e.emitForIn(s)
		default:
			e.emitForCond(s)
		}
	}
}

func (e *emitter) emitExpr(expr ast.Expr) string {
	switch ex := expr.(type) {
	case *ast.IntLit:
		return fmt.Sprintf("%d", ex.Value)
	case *ast.Ident:
		tmp := e.nextTmp()
		if e.ptrVars[ex.Name] {
			e.emit("  %s = load ptr, ptr %%%s", tmp, ex.Name)
		} else {
			e.emit("  %s = load i32, ptr %%%s", tmp, ex.Name)
		}
		return tmp
	case *ast.BinaryExpr:
		left := e.emitExpr(ex.X)
		right := e.emitExpr(ex.Y)
		tmp := e.nextTmp()
		switch ex.Op {
		case lexer.TokenPlus:
			e.emit("  %s = add i32 %s, %s", tmp, left, right)
		case lexer.TokenMinus:
			e.emit("  %s = sub i32 %s, %s", tmp, left, right)
		case lexer.TokenStar:
			e.emit("  %s = mul i32 %s, %s", tmp, left, right)
		case lexer.TokenSlash:
			e.emit("  %s = sdiv i32 %s, %s", tmp, left, right)
		case lexer.TokenEqEq:
			e.emit("  %s = icmp eq i32 %s, %s", tmp, left, right)
		case lexer.TokenBangEq:
			e.emit("  %s = icmp ne i32 %s, %s", tmp, left, right)
		case lexer.TokenLt:
			e.emit("  %s = icmp slt i32 %s, %s", tmp, left, right)
		case lexer.TokenLtEq:
			e.emit("  %s = icmp sle i32 %s, %s", tmp, left, right)
		case lexer.TokenGt:
			e.emit("  %s = icmp sgt i32 %s, %s", tmp, left, right)
		case lexer.TokenGtEq:
			e.emit("  %s = icmp sge i32 %s, %s", tmp, left, right)
		}
		return tmp
	case *ast.IfExpr:
		return e.emitIfExpr(ex)
	case *ast.CallExpr:
		fnName := ""
		if ident, ok := ex.Fn.(*ast.Ident); ok {
			fnName = ident.Name
		}
		
		if e.info != nil && e.info.Instances != nil {
			if inst, ok := e.info.Instances[ex]; ok {
				fnName = inst.Mangled
			}
		}
		var args []string
		
		if fnName == "print" {
			fnName = "puts"
			// String is returned as a ptr to {ptr, i32}. We need to load the first element (ptr).
			strStructPtr := e.emitExpr(ex.Args[0].Value)
			charPtr := e.nextTmp()
			e.emit("  %s = load ptr, ptr %s", charPtr, strStructPtr)
			args = append(args, fmt.Sprintf("ptr %s", charPtr))
		} else {
			for _, arg := range ex.Args {
				args = append(args, fmt.Sprintf("i32 %s", e.emitExpr(arg.Value)))
			}
		}

		tmp := e.nextTmp()
		e.emit("  %s = call i32 @%s(%s)", tmp, fnName, strings.Join(args, ", "))
		return tmp
	case *ast.FieldExpr:
		structPtr := e.emitExpr(ex.X)
		// We need the struct name to do getelementptr. 
		// Since we lack type info in this stub, we'll try to guess based on the field name.
		structName := "Unknown"
		idx := 0
		
		for sName, fMap := range e.structFields {
			if i, ok := fMap[ex.Name]; ok {
				structName = sName
				idx = i
				break
			}
		}

		fieldPtr := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, structName, structPtr, idx)
		tmp := e.nextTmp()
		e.emit("  %s = load i32, ptr %s", tmp, fieldPtr)
		return tmp
	case *ast.StructLit:
		tmp := e.nextTmp()
		structName := "Unknown"
		if named, ok := ex.Type.(*ast.NamedType); ok {
			structName = named.Name
		}
		e.emit("  %s = alloca %%%s", tmp, structName)
		
		fieldMap := e.structFields[structName]
		for _, field := range ex.Fields {
			val := e.emitExpr(field.Value)
			idx := 0
			if fieldMap != nil {
				idx = fieldMap[field.Name]
			}
			fieldPtr := e.nextTmp()
			e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, structName, tmp, idx)
			e.emit("  store i32 %s, ptr %s", val, fieldPtr)
		}
		return tmp
	case *ast.BlockExpr:
		return e.emitBlockExpr(ex)
	case *ast.MatchExpr:
		return e.emitMatchExpr(ex)
	case *ast.UnaryExpr:
		switch ex.Op {
		case lexer.TokenAmp:
			return e.emitAddrOf(ex.X)
		case lexer.TokenStar:
			return e.emitDeref(ex.X)
		default:
			return e.emitExpr(ex.X)
		}
	case *ast.ArrayLit:
		return e.emitArrayLit(ex)
	case *ast.IndexExpr:
		return e.emitIndexExpr(ex)
	case *ast.StringLit:
		return e.emitStringLit(ex)
	}
	return "0"
}

func (e *emitter) emitAddrOf(expr ast.Expr) string {
	switch ex := expr.(type) {
	case *ast.Ident:
		return "%" + ex.Name
	case *ast.FieldExpr:
		structPtr := e.emitExpr(ex.X)
		structName := "Unknown"
		idx := 0
		for sName, fMap := range e.structFields {
			if i, ok := fMap[ex.Name]; ok {
				structName = sName
				idx = i
				break
			}
		}
		fieldPtr := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 %d", fieldPtr, structName, structPtr, idx)
		return fieldPtr
	default:
		return e.emitExpr(expr)
	}
}

func (e *emitter) emitDeref(expr ast.Expr) string {
	switch ex := expr.(type) {
	case *ast.Ident:
		if e.ptrVars[ex.Name] {
			ptrVal := e.nextTmp()
			e.emit("  %s = load ptr, ptr %%%s", ptrVal, ex.Name)
			tmp := e.nextTmp()
			e.emit("  %s = load i32, ptr %s", tmp, ptrVal)
			return tmp
		}
		tmp := e.nextTmp()
		e.emit("  %s = load i32, ptr %%%s", tmp, ex.Name)
		return tmp
	default:
		ptr := e.emitExpr(expr)
		tmp := e.nextTmp()
		e.emit("  %s = load i32, ptr %s", tmp, ptr)
		return tmp
	}
}

func (e *emitter) emitArrayLit(al *ast.ArrayLit) string {
	switch lt := al.Type.(type) {
	case *ast.ArrayType:
		elemType := e.resolveType(lt.Elem)
		var length uint64 = uint64(len(al.Elems))
		if il, ok := lt.Len.(*ast.IntLit); ok {
			length = il.Value
		}
		arrType := fmt.Sprintf("[%d x %s]", length, elemType)
		tmp := e.nextTmp()
		e.emit("  %s = alloca %s", tmp, arrType)
		for i, elem := range al.Elems {
			val := e.emitExpr(elem)
			elemPtr := e.nextTmp()
			e.emit("  %s = getelementptr %s, ptr %s, i32 0, i32 %d", elemPtr, arrType, tmp, i)
			e.emit("  store %s %s, ptr %s", elemType, val, elemPtr)
		}
		return tmp
	case *ast.SliceType:
		elemType := e.resolveType(lt.Elem)
		n := len(al.Elems)
		sliceType := "{ ptr, i32, i32 }"
		tmp := e.nextTmp()
		e.emit("  %s = alloca %s", tmp, sliceType)
		if n > 0 {
			arrPtr := e.nextTmp()
			e.emit("  %s = alloca [%d x %s]", arrPtr, n, elemType)
			for i, elem := range al.Elems {
				val := e.emitExpr(elem)
				elemPtr := e.nextTmp()
				e.emit("  %s = getelementptr [%d x %s], ptr %s, i32 0, i32 %d", elemPtr, n, elemType, arrPtr, i)
				e.emit("  store %s %s, ptr %s", elemType, val, elemPtr)
			}
			dataField := e.nextTmp()
			e.emit("  %s = getelementptr %s, ptr %s, i32 0, i32 0", dataField, sliceType, tmp)
			e.emit("  store ptr %s, ptr %s", arrPtr, dataField)
		}
		lenField := e.nextTmp()
		e.emit("  %s = getelementptr %s, ptr %s, i32 0, i32 1", lenField, sliceType, tmp)
		e.emit("  store i32 %d, ptr %s", n, lenField)
		capField := e.nextTmp()
		e.emit("  %s = getelementptr %s, ptr %s, i32 0, i32 2", capField, sliceType, tmp)
		e.emit("  store i32 %d, ptr %s", n, capField)
		return tmp
	default:
		n := len(al.Elems)
		if n == 0 {
			return "0"
		}
		tmp := e.nextTmp()
		e.emit("  %s = alloca [%d x i32]", tmp, n)
		for i, elem := range al.Elems {
			val := e.emitExpr(elem)
			elemPtr := e.nextTmp()
			e.emit("  %s = getelementptr [%d x i32], ptr %s, i32 0, i32 %d", elemPtr, n, tmp, i)
			e.emit("  store i32 %s, ptr %s", val, elemPtr)
		}
		return tmp
	}
}

func (e *emitter) emitStringLit(sl *ast.StringLit) string {
	var text string
	if len(sl.Parts) > 0 && sl.Parts[0].Kind == ast.PartText {
		text = sl.Parts[0].Text
	}
	
	name, ok := e.stringLits[text]
	if !ok {
		name = fmt.Sprintf("@.str.%d", len(e.stringLits))
		e.stringLits[text] = name
	}
	
	length := len(text) + 1
	
	tmp := e.nextTmp()
	e.emit("  %s = alloca { ptr, i32 }", tmp)
	
	strPtr := e.nextTmp()
	e.emit("  %s = getelementptr [%d x i8], ptr %s, i32 0, i32 0", strPtr, length, name)
	
	ptrField := e.nextTmp()
	e.emit("  %s = getelementptr { ptr, i32 }, ptr %s, i32 0, i32 0", ptrField, tmp)
	e.emit("  store ptr %s, ptr %s", strPtr, ptrField)
	
	lenField := e.nextTmp()
	e.emit("  %s = getelementptr { ptr, i32 }, ptr %s, i32 0, i32 1", lenField, tmp)
	e.emit("  store i32 %d, ptr %s", len(text), lenField)
	
	return tmp
}

func (e *emitter) emitIndexExpr(ie *ast.IndexExpr) string {
	base := e.emitExpr(ie.X)
	idx := e.emitExpr(ie.Indices[0])
	baseName := ""
	if ident, ok := ie.X.(*ast.Ident); ok {
		baseName = ident.Name
	}
	elemType := "i32"
	if vt, ok := e.varTypes[baseName]; ok {
		if len(vt) > 4 && vt[0] == '[' {
			closeBracket := 0
			for i := 1; i < len(vt); i++ {
				if vt[i] == ']' {
					closeBracket = i
					break
				}
			}
			if closeBracket > 0 {
				elemType = vt[closeBracket+2:]
			}
		}
	}
	tmp := e.nextTmp()
	e.emit("  %s = getelementptr %s, ptr %s, i32 %s", tmp, elemType, base, idx)
	val := e.nextTmp()
	e.emit("  %s = load %s, ptr %s", val, elemType, tmp)
	return val
}

func (e *emitter) resolveType(t ast.Type) string {
	switch ty := t.(type) {
	case *ast.NamedType:
		switch ty.Name {
		case "int":
			return "i32"
		case "float":
			return "double"
		case "bool":
			return "i1"
		case "void":
			return "void"
		default:
			return "%" + ty.Name
		}
	case *ast.PointerType:
		inner := e.resolveType(ty.Elem)
		if inner == "void" {
			return "i8*"
		}
		return inner + "*"
	case *ast.SliceType:
		return "{ ptr, i32, i32 }"
	case *ast.ArrayType:
		if il, ok := ty.Len.(*ast.IntLit); ok {
			inner := e.resolveType(ty.Elem)
			return fmt.Sprintf("[%d x %s]", il.Value, inner)
		}
		return "i32*"
	default:
		return "i32"
	}
}
