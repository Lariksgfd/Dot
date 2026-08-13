package llvm

import (
	"strings"

	"github.com/dotlang/dot/ast"
)

func (e *emitter) emitMapLit(ex *ast.MapLit) string {
	res := e.nextTmp()
	e.emit("  %s = call ptr @dot_map_new()", res)
	for _, entry := range ex.Entries {
		key := e.emitExpr(entry.Key)
		val := e.emitExpr(entry.Value)
		
		keyPtr := e.boxValue(key, e.exprType(entry.Key))
		valPtr := e.boxValue(val, e.exprType(entry.Value))
		
		e.emit("  call void @dot_map_set(ptr %s, ptr %s, ptr %s)", res, keyPtr, valPtr)
	}
	return res
}

func (e *emitter) boxValue(val string, typ string) string {
	if isNamedAgg(typ) || typ == "ptr" {
		return val
	}
	// For primitives, we either cast to ptr or allocate and store.
	// For now, inttoptr / bitcast is simplest if assuming 64-bit space.
	if typ == "i64" {
		p := e.nextTmp()
		e.emit("  %s = inttoptr i64 %s to ptr", p, val)
		return p
	}
	if typ == "double" {
		i := e.nextTmp()
		e.emit("  %s = bitcast double %s to i64", i, val)
		p := e.nextTmp()
		e.emit("  %s = inttoptr i64 %s to ptr", p, i)
		return p
	}
	if typ == "i1" {
		i := e.nextTmp()
		e.emit("  %s = zext i1 %s to i64", i, val)
		p := e.nextTmp()
		e.emit("  %s = inttoptr i64 %s to ptr", p, i)
		return p
	}
	if typ == "i32" {
		i := e.nextTmp()
		e.emit("  %s = sext i32 %s to i64", i, val)
		p := e.nextTmp()
		e.emit("  %s = inttoptr i64 %s to ptr", p, i)
		return p
	}
	return val
}

func (e *emitter) emitSpawnExpr(ex *ast.SpawnExpr) string {
	fnName := e.nextLabel("spawn_fn")
	
	oldSb := e.sb
	oldVarTypes := e.varTypes
	oldPtrVars := e.ptrVars
	oldAllocas := e.varAllocas
	
	e.sb = strings.Builder{}
	// Not copying context properly for v0.1 as closures need a pass
	e.varTypes = make(map[string]string)
	e.ptrVars = make(map[string]bool)
	e.varAllocas = make(map[string]bindAlloca)
	
	e.emit("define ptr @%s(ptr %%ctx) {", fnName)
	e.emit("entry:")
	for _, stmt := range ex.Block.Stmts {
		e.emitStmt(stmt)
	}
	e.emit("  ret ptr null")
	e.emit("}")
	
	fnCode := e.sb.String()
	e.tasks = append(e.tasks, fnCode)
	
	e.sb = oldSb
	e.varTypes = oldVarTypes
	e.ptrVars = oldPtrVars
	e.varAllocas = oldAllocas
	
	res := e.nextTmp()
	e.emit("  %s = call ptr @dot_spawn(ptr @%s, ptr null)", res, fnName)
	return res
}

func (e *emitter) emitForIterator(s *ast.ForStmt) {
	iterVal := e.emitExpr(s.Iterable)
	t := e.info.TypeOf(s.Iterable)
	
	structName := e.namedTypeName(t)
	if structName == "" {
		structName = "Iterator"
	}
	nextFn := "Dot_" + structName + "_Next"
	
	loopVar := ""
	if ident, ok := s.Value.(*ast.Ident); ok {
		loopVar = ident.Name
	} else {
		loopVar = e.nextLabel("for.it")
	}
	
	elemType := "i64"
	loopAlloc := e.uniqueAllocaName(loopVar)
	e.varAllocas[loopVar] = bindAlloca{name: loopAlloc, ptr: false, typ: elemType}
	e.emit("  %%%s = alloca %s", loopAlloc, elemType)
	e.varTypes[loopVar] = elemType

	condBlock := e.nextLabel("for.cond")
	bodyBlock := e.nextLabel("for.body")
	endBlock := e.nextLabel("for.end")

	e.emit("  br label %%%s", condBlock)
	e.emit("\n%s:", condBlock)

	optVal := e.nextTmp()
	e.emit("  %s = call { i32, %s } @%s(ptr %s)", optVal, elemType, nextFn, iterVal)

	tag := e.nextTmp()
	e.emit("  %s = extractvalue { i32, %s } %s, 0", tag, elemType, optVal)
	
	isSome := e.nextTmp()
	e.emit("  %s = icmp eq i32 %s, 0", isSome, tag)
	e.emit("  br i1 %s, label %%%s, label %%%s", isSome, bodyBlock, endBlock)

	e.emit("\n%s:", bodyBlock)
	val := e.nextTmp()
	e.emit("  %s = extractvalue { i32, %s } %s, 1", val, elemType, optVal)
	e.emit("  store %s %s, ptr %%%s", elemType, val, loopAlloc)

	if s.Body != nil {
		for _, bs := range s.Body.Stmts {
			e.emitStmt(bs)
		}
	}
	e.emit("  br label %%%s", condBlock)

	e.emit("\n%s:", endBlock)
}
