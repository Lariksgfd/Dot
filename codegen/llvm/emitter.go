package llvm

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
	"github.com/dotlang/dot/types"
)

type emitter struct {
	info             *types.Info
	prog             *ast.Program
	sb               strings.Builder
	tmpID            int
	lblID            int
	structFields     map[string]map[string]int
	structFieldTypes map[string][]string
	enumVariants     map[string]map[string]int
	ptrVars          map[string]bool
	varTypes         map[string]string
	stringLits       map[string]string
	implMethods      map[string]*ast.FnDecl
	genericArgs      map[string]types.Type
	implRecv         string
	currentRetType   string
	selfName         string
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
		info:             info,
		prog:             prog,
		structFields:     make(map[string]map[string]int),
		structFieldTypes: make(map[string][]string),
		enumVariants:     make(map[string]map[string]int),
		ptrVars:          make(map[string]bool),
		varTypes:         make(map[string]string),
		stringLits:       make(map[string]string),
	}

	e.emit("declare void @dot_retain(ptr)")
	e.emit("declare void @dot_release(ptr)")
	e.emit("declare i32 @puts(ptr)")
	e.emit("declare i32 @printf(ptr, ...)")
	e.emit("declare ptr @malloc(i64)")
	e.emit("declare void @exit(i32)")
	e.emitRuntimeHelpers()

	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *ast.StructDecl:
			e.emitStructDecl(d)
		case *ast.EnumDecl:
			e.emitEnumDecl(d)
		}
	}

	for _, decl := range prog.Decls {
		if impl, ok := decl.(*ast.ImplDecl); ok {
			recvName := ""
			if named, ok := impl.Type.(*ast.NamedType); ok {
				recvName = named.Name
			}
			for _, m := range impl.Methods {
				if m.Body == nil && m.ExprBody == nil {
					continue
				}
				e.emitFn(m, "Dot_"+recvName+"_"+m.Name, recvName)
			}
		}
	}

	for _, decl := range prog.Decls {
		if fn, ok := decl.(*ast.FnDecl); ok {
			if len(fn.TypeParams) == 0 {
				e.emitFn(fn, fn.Name, "")
			}
		}
	}

	if e.info != nil {
		for _, inst := range e.info.InstanceList {
			if inst.Generic != nil && inst.Generic.Kind == types.SymFunc {
				if fn, ok := inst.Generic.Decl.(*ast.FnDecl); ok {
					e.genericArgs = make(map[string]types.Type)
					for i, tp := range fn.TypeParams {
						if i < len(inst.TypeArgs) {
							e.genericArgs[tp.Name] = inst.TypeArgs[i]
						}
					}
					e.emitFn(fn, inst.Mangled, "")
					e.genericArgs = nil
				}
			}
		}
	}

	var globalsSb strings.Builder
	globalsSb.WriteString("; ModuleID = 'main'\n")
	globalsSb.WriteString(fmt.Sprintf("source_filename = \"%s\"\n\n", prog.File))

	for content, name := range e.stringLits {
		length := len(content) + 1
		globalsSb.WriteString(fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\\00\"\n", name, length, llvmEscape(content)))
	}
	globalsSb.WriteString("\n")

	return globalsSb.String() + e.sb.String(), nil
}

func (e *emitter) emitStructDecl(sd *ast.StructDecl) {
	var fields []string
	fieldMap := make(map[string]int)
	fieldTypes := make([]string, 0, len(sd.Fields))
	for i, f := range sd.Fields {
		ft := e.structFieldLLVMType(f.Type)
		fields = append(fields, ft)
		fieldTypes = append(fieldTypes, ft)
		fieldMap[f.Name] = i
	}
	e.structFields[sd.Name] = fieldMap
	e.structFieldTypes[sd.Name] = fieldTypes
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

	matchType := e.exprType(me)
	if matchType == "void" {
		matchType = "i64"
	}
	if isNamedAgg(matchType) {
		matchType = "ptr"
	}

	resPtr := "%" + e.nextLabel("match.res")
	e.emit("  %s = alloca %s", resPtr, matchType)

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
	tagType := "i32"
	if enumName != "" {
		tagPtr := e.nextTmp()
		e.emit("  %s = getelementptr %%%s, ptr %s, i32 0, i32 0", tagPtr, enumName, subjectPtr)
		tagVal = e.nextTmp()
		e.emit("  %s = load i32, ptr %s", tagVal, tagPtr)
	} else {
		tagVal = subjectPtr
		tagType = e.exprType(me.Subject)
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
			if tagType == "{ ptr, i32 }" {
				branchCond = e.emitStringEq(tagVal, litVal)
			} else {
				cmpTmp := e.nextTmp()
				e.emit("  %s = icmp eq %s %s, %s", cmpTmp, tagType, tagVal, litVal)
				branchCond = cmpTmp
			}
		} else if _, ok := pat.(*ast.WildcardPattern); ok {
			bodyVal := e.emitExpr(arm.Body)
			e.emit("  store %s %s, ptr %s", matchType, bodyVal, resPtr)
			e.emit("  br label %%%s", endBlock)
			continue
		} else if ip, ok := pat.(*ast.IdentPattern); ok {
			e.emit("  %%%s = alloca %s", ip.Name, tagType)
			e.emit("  store %s %s, ptr %%%s", tagType, tagVal, ip.Name)
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
			e.emit("  store %s %s, ptr %s", matchType, bodyVal, resPtr)
			e.emit("  br label %%%s", endBlock)
			continue
		} else {
			bodyVal := e.emitExpr(arm.Body)
			e.emit("  store %s %s, ptr %s", matchType, bodyVal, resPtr)
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
		e.emit("  store %s %s, ptr %s", matchType, bodyVal, resPtr)
		e.emit("  br label %%%s", endBlock)
	}

	e.emit("\n%s:", failBlock)
	e.emit("  store %s %s, ptr %s", matchType, zeroConst(matchType), resPtr)
	e.emit("  br label %%%s", endBlock)

	e.emit("\n%s:", endBlock)
	tmp := e.nextTmp()
	e.emit("  %s = load %s, ptr %s", tmp, matchType, resPtr)
	return tmp
}

func (e *emitter) emitFn(fn *ast.FnDecl, name string, recvName string) {
	// Variable state is per-function: reset it so names don't leak
	// between functions.
	e.varTypes = make(map[string]string)
	e.ptrVars = make(map[string]bool)
	e.implRecv = recvName
	e.selfName = "self"

	retType := "void"
	if fn.Sig != nil && fn.Sig.Result != nil {
		if nt, ok := fn.Sig.Result.(*ast.NamedType); ok && e.isTypeParam(fn, nt.Name) {
			retType = e.instantiatedType(nt.Name)
		} else {
			retType = e.resolveType(fn.Sig.Result)
		}
	}
	// Keep main() as i32 so the CRT gets a proper exit code.
	if name == "main" && retType == "void" {
		retType = "i32"
	}
	e.currentRetType = retType
	sigRetType := retType
	if isNamedAgg(sigRetType) {
		sigRetType = "ptr"
	}

	var params []string
	if fn.Sig != nil && fn.Sig.Recv != nil && recvName != "" {
		e.selfName = fn.Sig.Recv.Name
		params = append(params, fmt.Sprintf("ptr %%%s.arg", e.selfName))
		e.varTypes[e.selfName] = "%" + recvName
		e.ptrVars[e.selfName] = true
	}
	if fn.Sig != nil {
		for _, p := range fn.Sig.Params {
			pt := "i64"
			if p.Type != nil {
				if nt, ok := p.Type.(*ast.NamedType); ok && e.isTypeParam(fn, nt.Name) {
					pt = e.instantiatedType(nt.Name)
				} else {
					pt = e.resolveType(p.Type)
				}
			}
			declPT := pt
			if isNamedAgg(pt) {
				declPT = "ptr"
				e.ptrVars[p.Name] = true
			}
			e.varTypes[p.Name] = pt
			params = append(params, fmt.Sprintf("%s %%%s.arg", declPT, p.Name))
		}
	}
	e.emit("define %s @%s(%s) {", sigRetType, name, strings.Join(params, ", "))
	e.emit("entry:")
	if fn.Sig != nil && fn.Sig.Recv != nil && recvName != "" {
		e.emit("  %%%s = alloca ptr", e.selfName)
		e.emit("  store ptr %%%s.arg, ptr %%%s", e.selfName, e.selfName)
	}
	if fn.Sig != nil {
		for _, p := range fn.Sig.Params {
			pt := e.varTypes[p.Name]
			declPT := pt
			if isNamedAgg(pt) {
				declPT = "ptr"
			}
			e.emit("  %%%s = alloca %s", p.Name, declPT)
			e.emit("  store %s %%%s.arg, ptr %%%s", declPT, p.Name, p.Name)
		}
	}
	if fn.ExprBody != nil {
		val := e.emitExpr(fn.ExprBody)
		if isNamedAgg(retType) {
			val = e.materializeStruct(strings.TrimPrefix(retType, "%"), val)
		}
		e.emit("  ret %s %s", sigRetType, val)
		e.emit("}")
		return
	}
	if fn.Body != nil {
		for _, stmt := range fn.Body.Stmts {
			e.emitStmt(stmt)
		}
	}
	if sigRetType == "void" {
		e.emit("  ret void")
	} else {
		zero := zeroConst(sigRetType)
		if sigRetType == "ptr" {
			zero = "null"
		}
		e.emit("  ret %s %s", sigRetType, zero)
	}
	e.emit("}")
}

func (e *emitter) isPtrValue(expr ast.Expr) bool {
	switch ex := expr.(type) {
	case *ast.UnaryExpr:
		return ex.Op == lexer.TokenAmp
	case *ast.Ident:
		return e.ptrVars[ex.Name]
	case *ast.ArrayLit, *ast.StructLit:
		return true
	default:
		return false
	}
}

func (e *emitter) emitStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		e.emitExpr(s.X)
	case *ast.ReturnStmt:
		if len(s.Values) > 0 {
			val := e.emitExpr(s.Values[0])
			t := e.exprType(s.Values[0])
			if isNamedAgg(t) || (e.currentRetType != "" && isNamedAgg(e.currentRetType)) {
				aggName := strings.TrimPrefix(t, "%")
				if aggName == "" || !isNamedAgg(t) {
					aggName = strings.TrimPrefix(e.currentRetType, "%")
				}
				val = e.materializeStruct(aggName, val)
				e.emit("  ret ptr %s", val)
			} else {
				e.emit("  ret %s %s", t, val)
			}
		} else {
			e.emit("  ret void")
		}
	case *ast.VarDecl:
		for i, nameExpr := range s.Names {
			if fe, ok := nameExpr.(*ast.FieldExpr); ok {
				// Assignment to a struct field: self.x = ...
				if i < len(s.Values) {
					val := e.emitExpr(s.Values[i])
					e.storeToField(fe, val)
				}
				continue
			}
			ident, ok := nameExpr.(*ast.Ident)
			if !ok {
				continue
			}
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
			needsARC := (declType != nil && types.IsHeap(declType) && !e.skipARC(declType)) || (expr != nil && e.info != nil && e.info.Escapes[expr] == types.Heap)

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

			llvmType := "i64"
			if s.Type != nil {
				llvmType = e.resolveType(s.Type)
			} else if vt, ok := e.varTypes[ident.Name]; ok {
				llvmType = vt
			} else if expr != nil {
				llvmType = e.exprType(expr)
			}

			if isNamedAgg(llvmType) {
				isPtr = true
			}

			if isPtr {
				if val == "0" {
					val = "null"
				}
				if !isReassign {
					e.ptrVars[ident.Name] = true
					e.emit("  %%%s = alloca ptr", ident.Name)
				}
				if shape := e.shapeType(expr); shape != "" {
					e.varTypes[ident.Name] = shape
				} else if isNamedAgg(llvmType) {
					e.varTypes[ident.Name] = llvmType
				}
				e.emit("  store ptr %s, ptr %%%s", val, ident.Name)
			} else {
				if !isReassign {
					e.varTypes[ident.Name] = llvmType
					e.emit("  %%%s = alloca %s", ident.Name, llvmType)
				}
				e.emit("  store %s %s, ptr %%%s", llvmType, val, ident.Name)
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
	case *ast.FloatLit:
		return floatConst(ex.Value)
	case *ast.BoolLit:
		if ex.Value {
			return "true"
		}
		return "false"
	case *ast.Ident:
		tmp := e.nextTmp()
		if e.ptrVars[ex.Name] {
			e.emit("  %s = load ptr, ptr %%%s", tmp, ex.Name)
		} else {
			t := "i64"
			if vt, ok := e.varTypes[ex.Name]; ok {
				t = vt
			} else {
				t = e.exprType(ex)
			}
			e.emit("  %s = load %s, ptr %%%s", tmp, t, ex.Name)
		}
		return tmp
	case *ast.BinaryExpr:
		left := e.emitExpr(ex.X)
		right := e.emitExpr(ex.Y)
		t := e.exprType(ex.X)
		if t == "{ ptr, i32 }" {
			switch ex.Op {
			case lexer.TokenPlus:
				return e.emitStringConcat(left, right)
			case lexer.TokenEqEq:
				return e.emitStringEq(left, right)
			case lexer.TokenBangEq:
				eq := e.emitStringEq(left, right)
				tmp := e.nextTmp()
				e.emit("  %s = xor i1 %s, true", tmp, eq)
				return tmp
			}
		}
		tmp := e.nextTmp()
		switch ex.Op {
		case lexer.TokenPlus:
			if isFloatType(t) {
				e.emit("  %s = fadd %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = add %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenMinus:
			if isFloatType(t) {
				e.emit("  %s = fsub %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = sub %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenStar:
			if isFloatType(t) {
				e.emit("  %s = fmul %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = mul %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenSlash:
			if isFloatType(t) {
				e.emit("  %s = fdiv %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = sdiv %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenPercent:
			if isFloatType(t) {
				e.emit("  %s = frem %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = srem %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenEqEq:
			if isFloatType(t) {
				e.emit("  %s = fcmp oeq %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = icmp eq %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenBangEq:
			if isFloatType(t) {
				e.emit("  %s = fcmp one %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = icmp ne %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenLt:
			if isFloatType(t) {
				e.emit("  %s = fcmp olt %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = icmp slt %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenLtEq:
			if isFloatType(t) {
				e.emit("  %s = fcmp ole %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = icmp sle %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenGt:
			if isFloatType(t) {
				e.emit("  %s = fcmp ogt %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = icmp sgt %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenGtEq:
			if isFloatType(t) {
				e.emit("  %s = fcmp oge %s %s, %s", tmp, t, left, right)
			} else {
				e.emit("  %s = icmp sge %s %s, %s", tmp, t, left, right)
			}
		case lexer.TokenAnd:
			e.emit("  %s = and i1 %s, %s", tmp, left, right)
		case lexer.TokenOr:
			e.emit("  %s = or i1 %s, %s", tmp, left, right)
		}
		return tmp
	case *ast.IfExpr:
		return e.emitIfExpr(ex)
	case *ast.CallExpr:
		return e.emitCallExpr(ex)
	case *ast.FieldExpr:
		return e.emitFieldExpr(ex)
	case *ast.StructLit:
		return e.emitStructLitAggr(ex)
	case *ast.BlockExpr:
		return e.emitBlockExpr(ex)
	case *ast.MatchExpr:
		return e.emitMatchExpr(ex)
	case *ast.SelfExpr:
		tmp := e.nextTmp()
		e.emit("  %s = load ptr, ptr %%%s", tmp, e.selfName)
		return tmp
	case *ast.UnaryExpr:
		switch ex.Op {
		case lexer.TokenAmp:
			return e.emitAddrOf(ex.X)
		case lexer.TokenStar:
			return e.emitDeref(ex.X)
		case lexer.TokenMinus:
			inner := e.emitExpr(ex.X)
			t := e.exprType(ex.X)
			tmp := e.nextTmp()
			if isFloatType(t) {
				e.emit("  %s = fneg %s %s", tmp, t, inner)
			} else {
				e.emit("  %s = sub %s 0, %s", tmp, t, inner)
			}
			return tmp
		case lexer.TokenNot:
			inner := e.emitExpr(ex.X)
			tmp := e.nextTmp()
			e.emit("  %s = xor i1 %s, true", tmp, inner)
			return tmp
		default:
			return e.emitExpr(ex.X)
		}
	case *ast.ArrayLit:
		return e.emitArrayLit(ex)
	case *ast.IndexExpr:
		return e.emitIndexExpr(ex)
	case *ast.AssignExpr:
		return e.emitAssignExpr(ex)
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
	t := e.exprType(expr)
	switch ex := expr.(type) {
	case *ast.Ident:
		if e.ptrVars[ex.Name] {
			ptrVal := e.nextTmp()
			e.emit("  %s = load ptr, ptr %%%s", ptrVal, ex.Name)
			tmp := e.nextTmp()
			e.emit("  %s = load %s, ptr %s", tmp, t, ptrVal)
			return tmp
		}
		tmp := e.nextTmp()
		e.emit("  %s = load %s, ptr %%%s", tmp, t, ex.Name)
		return tmp
	default:
		ptr := e.emitExpr(expr)
		tmp := e.nextTmp()
		e.emit("  %s = load %s, ptr %s", tmp, t, ptr)
		return tmp
	}
}

func (e *emitter) emitArrayLit(al *ast.ArrayLit) string {
	switch lt := al.Type.(type) {
	case *ast.ArrayType:
		elemType := e.resolveType(lt.Elem)
		storedElem := elemType
		if isNamedAgg(elemType) {
			storedElem = "ptr"
		}
		var length uint64 = uint64(len(al.Elems))
		if il, ok := lt.Len.(*ast.IntLit); ok {
			length = il.Value
		}
		arrType := fmt.Sprintf("[%d x %s]", length, storedElem)
		tmp := e.nextTmp()
		e.emit("  %s = alloca %s", tmp, arrType)
		for i, elem := range al.Elems {
			val := e.emitExpr(elem)
			elemPtr := e.nextTmp()
			e.emit("  %s = getelementptr %s, ptr %s, i32 0, i32 %d", elemPtr, arrType, tmp, i)
			e.emit("  store %s %s, ptr %s", storedElem, val, elemPtr)
		}
		return tmp
	case *ast.SliceType:
		elemType := e.resolveType(lt.Elem)
		storedElem := elemType
		if isNamedAgg(elemType) {
			storedElem = "ptr"
		}
		n := len(al.Elems)
		sliceType := "{ ptr, i32, i32 }"
		tmp := e.nextTmp()
		e.emit("  %s = alloca %s", tmp, sliceType)
		if n > 0 {
			arrPtr := e.nextTmp()
			e.emit("  %s = alloca [%d x %s]", arrPtr, n, storedElem)
			for i, elem := range al.Elems {
				val := e.emitExpr(elem)
				elemPtr := e.nextTmp()
				e.emit("  %s = getelementptr [%d x %s], ptr %s, i32 0, i32 %d", elemPtr, n, storedElem, arrPtr, i)
				e.emit("  store %s %s, ptr %s", storedElem, val, elemPtr)
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
		e.emit("  %s = alloca [%d x i64]", tmp, n)
		for i, elem := range al.Elems {
			val := e.emitExpr(elem)
			elemPtr := e.nextTmp()
			e.emit("  %s = getelementptr [%d x i64], ptr %s, i32 0, i32 %d", elemPtr, n, tmp, i)
			e.emit("  store i64 %s, ptr %s", val, elemPtr)
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

	// Return the fat pointer by value: { ptr, i32 } is a value-flow type in
	// this backend, so string literals yield the loaded aggregate.
	loadTmp := e.nextTmp()
	e.emit("  %s = load { ptr, i32 }, ptr %s", loadTmp, tmp)
	return loadTmp
}

func (e *emitter) emitIndexExpr(ie *ast.IndexExpr) string {
	info := e.resolveIndex(ie)
	if info.dataPtr == "" {
		// No checked type info: fall back to a plain gep on the base pointer.
		base := e.emitExpr(ie.X)
		idx := e.emitExpr(ie.Indices[0])
		idxType := e.exprType(ie.Indices[0])
		baseName := ""
		if ident, ok := ie.X.(*ast.Ident); ok {
			baseName = ident.Name
		}
		elemType := "i64"
		if vt, ok := e.varTypes[baseName]; ok {
			if i := strings.Index(vt, "]"); i >= 0 && i+1 < len(vt) {
				elemType = strings.TrimSpace(vt[i+1:])
			}
		}
		tmp := e.nextTmp()
		e.emit("  %s = getelementptr %s, ptr %s, %s %s", tmp, elemType, base, idxType, idx)
		val := e.nextTmp()
		e.emit("  %s = load %s, ptr %s", val, elemType, tmp)
		return val
	}
	if info.lenVal == "" {
		idx := e.emitExpr(ie.Indices[0])
		idxType := e.exprType(ie.Indices[0])
		tmp := e.nextTmp()
		e.emit("  %s = getelementptr %s, ptr %s, %s %s", tmp, info.elemType, info.dataPtr, idxType, idx)
		val := e.nextTmp()
		e.emit("  %s = load %s, ptr %s", val, info.elemType, tmp)
		return val
	}

	okBlock := e.nextLabel("idx.ok")
	oobBlock := e.nextLabel("idx.oob")
	endBlock := e.nextLabel("idx.end")
	idxType := e.exprType(ie.Indices[0])
	idx := e.emitExpr(ie.Indices[0])
	e.emitBoundsCheck(idx, idxType, info.lenVal, okBlock, oobBlock)

	e.emit("\n%s:", okBlock)
	elemPtr := e.nextTmp()
	e.emit("  %s = getelementptr %s, ptr %s, %s %s", elemPtr, info.elemType, info.dataPtr, idxType, idx)
	val := e.nextTmp()
	e.emit("  %s = load %s, ptr %s", val, info.elemType, elemPtr)
	e.emit("  br label %%%s", endBlock)

	e.emit("\n%s:", oobBlock)
	e.emit("  call void @dot_bounds_panic()")
	e.emit("  unreachable")

	e.emit("\n%s:", endBlock)
	res := e.nextTmp()
	e.emit("  %s = phi %s [ %s, %%%s ]", res, info.elemType, val, okBlock)
	return res
}

// indexInfo describes a resolved indexing container.
type indexInfo struct {
	dataPtr  string
	lenVal   string
	elemType string
}

// resolveIndex resolves the data pointer, length and element type for an
// index expression. dataPtr is "" when the container type is unknown.
func (e *emitter) resolveIndex(ie *ast.IndexExpr) *indexInfo {
	info := &indexInfo{elemType: "i64"}
	if e.info == nil {
		return info
	}
	switch tt := types.Underlying(e.info.TypeOf(ie.X)).(type) {
	case *types.Slice:
		info.elemType = e.llvmType(tt.Elem)
		if isNamedAgg(info.elemType) {
			info.elemType = "ptr"
		}
		base := e.emitExpr(ie.X)
		f0 := e.nextTmp()
		e.emit("  %s = getelementptr { ptr, i32, i32 }, ptr %s, i32 0, i32 0", f0, base)
		info.dataPtr = e.nextTmp()
		e.emit("  %s = load ptr, ptr %s", info.dataPtr, f0)
		f1 := e.nextTmp()
		e.emit("  %s = getelementptr { ptr, i32, i32 }, ptr %s, i32 0, i32 1", f1, base)
		l32 := e.nextTmp()
		e.emit("  %s = load i32, ptr %s", l32, f1)
		info.lenVal = e.nextTmp()
		e.emit("  %s = sext i32 %s to i64", info.lenVal, l32)
	case *types.Array:
		info.elemType = e.llvmType(tt.Elem)
		if isNamedAgg(info.elemType) {
			info.elemType = "ptr"
		}
		info.dataPtr = e.emitExpr(ie.X)
		info.lenVal = fmt.Sprintf("%d", tt.Len)
	case *types.Basic:
		if tt.Kind() == types.KindString {
			v := e.emitExpr(ie.X)
			p := e.nextTmp()
			e.emit("  %s = extractvalue { ptr, i32 } %s, 0", p, v)
			l := e.nextTmp()
			e.emit("  %s = extractvalue { ptr, i32 } %s, 1", l, v)
			info.dataPtr = p
			info.elemType = "i8"
			info.lenVal = e.nextTmp()
			e.emit("  %s = sext i32 %s to i64", info.lenVal, l)
		}
	}
	return info
}

// emitBoundsCheck branches to okBlock when 0 <= idx < len and to oobBlock
// otherwise. lenVal must already be an i64 register or constant.
func (e *emitter) emitBoundsCheck(idx, idxType, lenVal, okBlock, oobBlock string) {
	ge := e.nextTmp()
	e.emit("  %s = icmp sge %s %s, 0", ge, idxType, idx)
	lt := e.nextTmp()
	e.emit("  %s = icmp slt %s %s, %s", lt, idxType, idx, lenVal)
	ok := e.nextTmp()
	e.emit("  %s = and i1 %s, %s", ok, ge, lt)
	e.emit("  br i1 %s, label %%%s, label %%%s", ok, okBlock, oobBlock)
}

func (e *emitter) resolveType(t ast.Type) string {
	switch ty := t.(type) {
	case *ast.NamedType:
		switch ty.Name {
		case "int", "uint", "uint64":
			return "i64"
		case "int8", "uint8", "byte":
			return "i8"
		case "int16", "uint16":
			return "i16"
		case "int32", "uint32":
			return "i32"
		case "float", "float64":
			return "double"
		case "float32":
			return "float"
		case "bool":
			return "i1"
		case "void":
			return "void"
		case "string":
			return "{ ptr, i32 }"
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
		return "i64*"
	default:
		return "i64"
	}
}
