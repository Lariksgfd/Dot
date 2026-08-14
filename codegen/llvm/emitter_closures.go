package llvm

import (
	"fmt"
	"strings"

	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/types"
)

func (e *emitter) emitFnLit(ex *ast.FnLit) string {
	captures := e.info.Captures[ex]
	
	// Create the closure struct: { env, fn }
	// We'll return a pointer to this struct or by value? 
	// For LLVM, let's build the env struct if there are captures.

	fnName := fmt.Sprintf("_dot_closure_%d", e.tmpID)
	e.tmpID++

	var params []string
	params = append(params, "ptr %_env")
	
	var argNames []string
	if ex.Sig != nil {
		for _, p := range ex.Sig.Params {
			pt := "i64"
			if p.Type != nil {
				pt = e.resolveType(p.Type)
			} else if e.info != nil {
				if sym, ok := e.info.Defs[p]; ok && sym.Type != nil {
					pt = e.llvmType(sym.Type)
				}
			}
			declPT := pt
			if isNamedAgg(pt) {
				declPT = "ptr"
			}
			params = append(params, fmt.Sprintf("%s %%%s.arg", declPT, p.Name))
			argNames = append(argNames, p.Name)
		}
	}

	retType := "void"
	fnType := e.info.TypeOf(ex)
	if ft, ok := fnType.(*types.Fn); ok && ft.Result != nil {
		retType = e.llvmType(ft.Result)
	}
	
	sigRetType := retType
	if isNamedAgg(sigRetType) {
		sigRetType = "ptr"
	}

	// Build env struct type
	envStructType := "{}"
	var envFields []string
	if len(captures) > 0 {
		var fields []string
		for _, cap := range captures {
			ct := e.llvmType(cap.Type)
			if isNamedAgg(ct) {
				ct = "ptr"
			}
			fields = append(fields, ct)
			envFields = append(envFields, ct)
		}
		envStructType = fmt.Sprintf("{ %s }", strings.Join(fields, ", "))
	}

	oldSb := e.sb
	oldVarTypes := e.varTypes
	oldPtrVars := e.ptrVars
	oldVarAllocas := e.varAllocas
	oldEmittedAllocas := e.emittedAllocas
	oldBindingID := e.bindingID
	oldRetType := e.currentRetType

	e.sb = strings.Builder{}
	e.varTypes = make(map[string]string)
	e.ptrVars = make(map[string]bool)
	e.varAllocas = make(map[string]bindAlloca)
	e.emittedAllocas = make(map[string]bool)
	e.bindingID = 0
	e.currentRetType = retType

	e.emit("define %s @%s(%s) {", sigRetType, fnName, strings.Join(params, ", "))
	e.emit("entry:")

	if len(captures) > 0 {
		e.emit("  %%env = bitcast ptr %%_env to ptr")
		for i, cap := range captures {
			allocName := e.uniqueAllocaName(cap.Name)
			capType := envFields[i]
			e.varAllocas[cap.Name] = bindAlloca{name: allocName, ptr: isNamedAgg(capType), typ: capType}
			e.ptrVars[cap.Name] = isNamedAgg(capType)
			e.varTypes[cap.Name] = capType
			e.emit("  %%%s = alloca %s", allocName, capType)
			valTmp := e.nextTmp()
			e.emit("  %s = getelementptr %s, ptr %%env, i32 0, i32 %d", valTmp, envStructType, i)
			loadTmp := e.nextTmp()
			e.emit("  %s = load %s, ptr %s", loadTmp, capType, valTmp)
			e.emit("  store %s %s, ptr %%%s", capType, loadTmp, allocName)
		}
	}

	if ex.Sig != nil {
		for i, p := range ex.Sig.Params {
			pt := "i64"
			if i+1 < len(params) {
				parts := strings.Split(params[i+1], " ")
				if len(parts) > 0 {
					pt = parts[0]
				}
			}
			allocName := e.uniqueAllocaName(p.Name)
			e.varAllocas[p.Name] = bindAlloca{name: allocName, ptr: isNamedAgg(pt), typ: pt}
			e.ptrVars[p.Name] = isNamedAgg(pt)
			e.varTypes[p.Name] = pt
			e.emit("  %%%s = alloca %s", allocName, pt)
			e.emit("  store %s %%%s.arg, ptr %%%s", pt, p.Name, allocName)
		}
	}

	if ex.ExprBody != nil {
		val := e.emitExpr(ex.ExprBody)
		if isNamedAgg(retType) {
			val = e.materializeAgg(strings.TrimPrefix(retType, "%%"), val)
		}
		e.emit("  ret %s %s", sigRetType, val)
	} else if ex.Body != nil {
		for _, stmt := range ex.Body.Stmts {
			e.emitStmt(stmt)
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
	} else {
		if sigRetType == "void" {
			e.emit("  ret void")
		} else {
			e.emit("  ret %s zeroinitializer", sigRetType)
		}
	}
	e.emit("}")

	funcCode := e.sb.String()
	e.sb = oldSb
	e.varTypes = oldVarTypes
	e.ptrVars = oldPtrVars
	e.varAllocas = oldVarAllocas
	e.emittedAllocas = oldEmittedAllocas
	e.bindingID = oldBindingID
	e.currentRetType = oldRetType

	e.tasks = append(e.tasks, funcCode)

	closureType := "{ ptr, ptr }"
	tmp := e.nextTmp()
	e.emit("  %s = alloca %s", tmp, closureType)

	envPtr := "null"
	if len(captures) > 0 {
		envPtrTmp := e.nextTmp()
		e.emit("  %s = call ptr @malloc(i64 80)", envPtrTmp) // 80 is a safe overestimation, should compute actual size
		envPtr = envPtrTmp
		for i, cap := range captures {
			val := "0"
			capType := envFields[i]
			if e.ptrVars[cap.Name] {
				loadTmp := e.nextTmp()
				e.emit("  %s = load ptr, ptr %%%s", loadTmp, e.allocaName(cap.Name))
				val = loadTmp
			} else {
				loadTmp := e.nextTmp()
				e.emit("  %s = load %s, ptr %%%s", loadTmp, capType, e.allocaName(cap.Name))
				val = loadTmp
			}
			fieldPtr := e.nextTmp()
			e.emit("  %s = getelementptr %s, ptr %s, i32 0, i32 %d", fieldPtr, envStructType, envPtr, i)
			e.emit("  store %s %s, ptr %s", capType, val, fieldPtr)
		}
	}

	fnPtrField := e.nextTmp()
	e.emit("  %s = getelementptr %s, ptr %s, i32 0, i32 0", fnPtrField, closureType, tmp)
	e.emit("  store ptr @%s, ptr %s", fnName, fnPtrField)

	envPtrField := e.nextTmp()
	e.emit("  %s = getelementptr %s, ptr %s, i32 0, i32 1", envPtrField, closureType, tmp)
	e.emit("  store ptr %s, ptr %s", envPtr, envPtrField)

	res := e.nextTmp()
	e.emit("  %s = load %s, ptr %s", res, closureType, tmp)
	return res
}
