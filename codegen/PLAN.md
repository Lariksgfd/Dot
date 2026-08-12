# Phase 5: C Code Generation — Plan

Translates a typed Dot AST into C source. Consumes `*types.Info` from Phase 4,
emits a self-contained `.c` file that links against `runtime/`.

---

## 1. File Breakdown

| File | Purpose | Est. lines |
|------|---------|------------|
| `codegen/codegen.go` | Entry point `Generate(info, prog)`, header includes, top-level driver, runtime footer | ~120 |
| `codegen/types.go` | Dot type -> C type string mapping, name mangling for monomorphised generics | ~180 |
| `codegen/emit_expr.go` | Expression codegen: literals, unary/binary, call, field, index, cast, match, if-expr, spawn, await, try | ~380 |
| `codegen/emit_stmt.go` | Statement codegen: var decl, assign, if, for, return, defer, break/continue, perf block | ~350 |
| `codegen/emit_decl.go` | Declaration codegen: struct/enum/trait forward decls + definitions, user functions, methods | ~380 |
| `codegen/emit_arc.go` | ARC helpers: retain/release emission, defer tracking, scope cleanup | ~200 |
| `codegen/builtins.go` | Builtin function C mappings (`print`, `len`, `assert`, `panic`, `type_of`, `input`) | ~150 |
| `codegen/codegen_test.go` | Tests: golden C output for each phase of codegen | ~250 |

Total: ~2010 lines across 8 files. Every file under 400 lines.

---

## 2. Type Mapping (Dot -> C)

Implemented in `codegen/types.go`.

### Primitives

| Dot | C | Notes |
|-----|---|-------|
| `int` | `int64_t` | |
| `int8` | `int8_t` | |
| `int16` | `int16_t` | |
| `int32` | `int32_t` | |
| `uint` | `uint64_t` | |
| `uint8` | `uint8_t` | |
| `uint16` | `uint16_t` | |
| `uint32` | `uint32_t` | |
| `float` | `double` | |
| `float32` | `float` | |
| `bool` | `bool` | `<stdbool.h>` |
| `byte` | `uint8_t` | alias of uint8 |
| `rune` | `int32_t` | |
| `string` | `DotString*` | ref-counted |
| `void` | `void` | |
| `never` | `void` | `panic()` diverges, body is `void` |

### Composites

| Dot | C | Notes |
|-----|---|-------|
| `[]T` | `DotSlice*` | ref-counted, `DotSlice` struct in runtime |
| `[N]T` | `struct { T data[N]; int64_t len; }` | fixed array as struct |
| `map[K]V` | `DotMap*` | ref-counted hash map |
| `(A, B, ...)` | `struct { A _0; B _1; ...; }` | anonymous tuple struct |
| `fn(A) -> R` | `R (*)(A, ...)` | function pointer |
| `*T` | `T*` | raw pointer (only inside `@perf`) |
| `dyn Trait` | `DotDyn*` | fat pointer: `{ void* data; void* vtable; }` |
| `weak T` | `DotWeak*` | weak reference |
| `Channel[T]` | `DotChannel*` | ref-counted channel |
| `Future[T]` | `DotFuture*` | ref-counted future handle |

### Named types

| Dot | C | Notes |
|-----|---|---|
| `struct Foo { x int; }` | `struct DotFoo { int64_t x; }` | prefix `Dot` avoids C namespace clashes |
| `enum Color { Red, Green }` | tagged union: `struct DotColor { int32_t tag; union { ... }; }` | unit-only enums could be `enum` but we use tagged union uniformly for simplicity |
| `enum Shape { Circle(float) }` | tagged union with payload in the union arm | |
| `trait Printable` | vtable struct `DotPrintable_vtable` + `DotPrintable_dyn` fat ptr | each impl gets its own vtable instance |
| `Option[T]` | `struct DotOption_T { int32_t tag; union { T value; }; }` | monomorphised per T |
| `Result[T,E]` | `struct DotResult_T_E { int32_t tag; union { T value; E error; }; }` | |

### Name mangling for monomorphised generics

Pattern: `Dot{BaseType}_{TypeArg1}_{TypeArg2}`

- `Stack[int]` -> `DotStack_int`
- `Result[int, Error]` -> `DotResult_int_DotError`
- `Option[string]` -> `DotOption_DotString_star` (string is `DotString*`)

The `Instance.Mangled` field (populated by Phase 4 / `generic.go`) already holds
this. When codegen encounters an `*ast.IndexExpr` on a Fn (generic instantiation
`Stack[int]`), it looks up `info.Instances[expr]` and uses `Mangled`.

---

## 3. Expression Codegen

Implemented in `codegen/emit_expr.go`. Each function returns the C expression
string. Heap/ARC info from `info.Heap` and `types.IsHeap(t)` drives retain/release.

### Literals
- `IntLit` -> `"42"` (no suffix; `int64_t` context handles it)
- `FloatLit` -> `"3.14"` (or `"3.14f"` for float32)
- `BoolLit` -> `"true"` / `"false"`
- `StringLit` -> `dot_string_from_lit("hello", 5)` (runtime call; heap-allocated `DotString*`)
- `RuneLit` -> not in v0.1 (no rune literals)
- `NilLit` -> context-dependent: `NULL` for pointers/weak, `DotNone` tag for Option

### Ident
- `x` -> lookup `info.Uses[ident]`; if local var, emit mangled C name (e.g., `x`). If
  parameter, emit as-is. If global/function, emit mangled global name.

### UnaryExpr
- `-x` -> `(-x)`
- `not x` -> `(!x)`
- `~x` -> `(~x)`
- `+x` -> `(x)` (no-op)

### BinaryExpr
- Arithmetic: `+`, `-`, `*`, `/`, `%` -> direct C operator
- `**` (power) -> `dot_pow_int(a, b)` / `dot_pow_float(a, b)` runtime call (no C `**`)
- Comparison: `==`, `!=`, `<`, `>`, `<=`, `>=` -> direct C operator
- Logical: `and` -> `&&`, `or` -> `||` (short-circuit, emitted as C `&&`/`||`)
- Bitwise: `&`, `|`, `^`, `<<`, `>>` -> direct C operator

### CallExpr
- Regular call: `f(arg0, arg1, ...)`
- Method call (Fn is `*FieldExpr`): `dot_Point_distance(&self, other)` (receiver
  passed as pointer for `self`/`mut self`, by value for non-mut non-pointer)
- Generic instantiation: look up `info.Instances[callExpr]`, emit mangled name
- Builtin: dispatch to `codegen/builtins.go` for special handling
- Default params: use `info.Calls[callExpr].ArgOrder` and `.Defaults` to fill
  missing args from param defaults
- Variadic: pack trailing args via runtime `DotSlice*` construction

### FieldExpr
- Struct field: `self.x` or `ptr->x` (auto-deref based on X's type)
- Tuple index: `t._0`, `t._1`
- Enum variant (constructor): emit tagged union literal
- Builtin property (`items.len`): runtime call `dot_slice_len(items)`
- Builtin method (`items.push(x)`): runtime call `dot_slice_push(items, x)`

### IndexExpr
- Slice index: `dot_slice_get(arr, i)` (bounds-checked)
- Array index: `arr.data[i]`
- Generic type instantiation: handled via `info.Instances`

### SliceExpr
- `items[1..3]` -> `dot_slice_sub(arr, 1, 3, false)` (exclusive end)
- `items[1..=3]` -> `dot_slice_sub(arr, 1, 3, true)` (inclusive)

### CastExpr (`as`)
- Numeric -> numeric: C cast `(int64_t)x`
- `string` -> `[]byte`: runtime `dot_string_to_bytes(s)`
- `[]byte` -> `string`: runtime `dot_string_from_bytes(b)`
- `T` -> `dyn Trait`: build fat pointer `{ .data = dot_retain(t), .vtable = &DotT_Trait_vtable }`

### IfExpr
- `if cond { a } else { b }` as ternary: `(cond) ? (a) : (b)` (single expr each branch)
- Multi-statement branches: emit as `if` statement + temporary variable assignment

### MatchExpr
- Enum match: `switch (subj.tag) { case 0: ...; case 1: ...; }`
- Literal match: `if/else if` chain
- Type match: not in v0.1 (requires RTTI); emit `dot_panic("type match not in v0.1")`
- Destructuring: extract union fields into fresh local vars

### PipeExpr
- Rewritten by Phase 4 to `f(x, y)` form; codegen just emits the rewritten call.

### TryExpr (`?`)
- `expr?` on Option: `({ DotOption* _tmp = expr; if (_tmp->tag == 1) return None; _tmp->value; })`
- `expr?` on Result: `({ DotResult* _tmp = expr; if (_tmp->tag == 1) return Err(_tmp->error); _tmp->value; })`
- The `return` is wrapped to return from the enclosing function (checked against `info.fnStack`).

### AwaitExpr
- `await future` -> `dot_future_get(future)` (blocks until result available)

### SpawnExpr
- `spawn { ... }` -> `dot_spawn(fn, ctx)` returns `DotFuture*`

### BlockExpr
- Used in match arms, lambda bodies. Emits as compound statement `({ ...; last_expr; })`.

### FnLit (lambda)
- `fn(x) { x * 2 }` -> closure struct: `DotClosure* c = dot_closure_new(fn_ptr, captures...)`;
  calling is `dot_closure_call(c, args...)`. Captured variables stored in closure struct.

---

## 4. Statement Codegen

Implemented in `codegen/emit_stmt.go`.

### VarDecl
- `x = 42` -> `int64_t x = 42;` (inferred type)
- `count int = 0` -> `int64_t count = 0;`
- `const PI = 3.14` -> `#define DOT_PI 3.14` (if compile-time constant) or `static const double PI = 3.14;`
- `a, b = 1, 2` -> multi-var: emit sequentially
- `_, err = f()` -> `_` becomes `_dot_unused_N` (compiler-generated discard)
- Heap-allocated init value: `int64_t x = dot_retain(expr);` (ARC ownership transfer)

### AssignExpr
- `x = val` -> `dot_release(x); x = dot_retain(val);` (if x is heap type)
- Compound: `x += val` -> `x += val` (no ARC needed for primitives)
- Multi-target: `a, b = 1, 2` -> emit sequentially or via tuple unpacking

### IfStmt
- `if cond { ... } else if cond2 { ... } else { ... }` -> direct C `if/else if/else`
- No parentheses around cond (Dot rule #3)

### ForStmt
- `for { }` -> `for (;;) { ... }`
- `for x > 0 { }` -> `while (x > 0) { ... }`
- `for i in 0..10` -> `for (int64_t i = 0; i < 10; i++) { ... }`
- `for i, item in items` -> iterator-based: `for (DotIter _it = dot_iter(items); dot_iter_next(&_it, &item, &i)) { ... }`
- `for k, v in map` -> map iterator similar to above
- Labeled loops: `DotLabel_outer: for (...) { ... break DotLabel_outer; ... }`

### ReturnStmt
- `return` -> `return;` (void)
- `return x` -> `return dot_retain_if_heap(x);` (caller receives ownership)
- `return a, b` -> `return (DotTuple_IntError){ . _0 = a, . _1 = b };`
- Defers execute before return: emit all registered defers, then `return`

### BreakStmt / ContinueStmt
- `break` -> `break;`
- `break @outer` -> `goto DotLabel_outer_break;` (label placed after loop)
- `continue` / `continue @label` -> `continue;` / `goto DotLabel_outer_continue;`

### DeferStmt
- `defer file.close()` -> register in per-function defer stack; emit at function exit
  and before each `return`. Runtime: `dot_defer_push(fn, ctx)` / `dot_defer_run(scope)`.

### PerfBlock
- `@perf { ... }` -> arena allocation: `DotArena* _arena = dot_arena_new(size); ...
  dot_arena_free(_arena);`. No ARC inside — all allocs are arena-bumped.

### ExprStmt
- Statement-position `if`, `match`, `spawn`: emit as the corresponding statement form.

---

## 5. Function Codegen

Implemented in `codegen/emit_decl.go`.

### Regular function
```
// Dot:
fn add(a int, b int) -> int {
    return a + b;
}

// C:
int64_t Dot_add(int64_t a, int64_t b) {
    return (a + b);
}
```

### Short-form function
```
// Dot: fn double(x int) -> int = x * 2
// C:
int64_t Dot_double(int64_t x) {
    return (x * 2);
}
```

### Method
```
// Dot: impl Point { fn distance(self, other Point) -> float { ... } }
// C:
double Dot_Point_distance(DotPoint* self, DotPoint other) {
    ...
}
```
- `self` is passed as `DotPoint*` (pointer, ARC-managed)
- `mut self` is also `DotPoint*` (mutability enforced by checker, not C)

### Static method
```
// Dot: fn origin() -> Point { ... }
// C:
DotPoint* Dot_Point_origin(void) {
    DotPoint* _result = dot_alloc(sizeof(DotPoint));
    ...
    return _result;  // caller owns
}
```

### Constructor convention
- `Point.new(x, y)` -> `DotPoint* Dot_Point_new(double x, double y)` (static method)

### Generic function monomorphisation
- `info.InstanceList` is the source of truth. For each `Instance`:
  1. Look up `Instance.Mangled` for the C name
  2. Substitute `Instance.TypeArgs` for `Instance.Generic.TypeParams` in the body
  3. Emit the instantiated function with concrete types
- Example: `max[int]` and `max[float]` become two separate C functions:
  `int64_t Dot_max_int(int64_t a, int64_t b)` and `double Dot_max_float(double a, double b)`

### Generic struct monomorphisation
- `Stack[int]` -> `struct DotStack_int { DotSlice* items; }`
- Each instantiated generic struct becomes a separate C struct definition

### Entry point (`main`)
- Dot `fn main()` -> C `int main(void) { ... dot_atexit_cleanup(); return 0; }`
- Top-level statements (ExprStmt at Program level) go inside `main`

---

## 6. ARC / Memory Management

Implemented in `codegen/emit_arc.go`.

### Rules (from SPEC §13 + Info.Heap)

1. **Retain** when: assigning a heap value to a new location, passing as argument
   (callee retains if needed), storing in a struct/slice/map field.
2. **Release** when: a heap variable goes out of scope, reassigning a heap variable,
   function exit (all owned locals).
3. **No ARC** for: stack-allocated values (compiler-decided via escape analysis),
   inside `@perf` blocks (arena), primitive types.

### ARC functions (runtime)

| Operation | C call |
|-----------|--------|
| Create string | `dot_string_from_lit(ptr, len)` -> `DotString*` |
| Retain | `dot_retain(DotRefcnt* obj)` |
| Release | `dot_release(DotRefcnt* obj)` -> frees if refcount==0 |
| Weak ref | `dot_weak_new(target)` / `dot_weak_upgrade(weak)` -> `Option[T]` |
| Arena alloc | `dot_arena_alloc(arena, size)` -> `void*` |

### Insertion strategy

- **Function entry**: no ARC setup needed (params are owned by caller or borrowed)
- **VarDecl with heap init**: `T* x = dot_retain(init_val);` (init_val's refcount is
  transferred; if it's a fresh allocation it already has count=1, no extra retain)
- **Assignment to heap var**: `dot_release(x); x = dot_retain(new_val);`
- **Function exit**: release all owned heap locals (tracked in per-function scope list)
- **Return**: retain the return value if it's heap (caller will release); don't release
  locals that were returned (ownership transferred)

### defer interaction
- Defers run before return and before scope exit
- Each defer call is emitted as `dot_defer_register(scope, fn, ctx)` at the defer site
- At scope exit: `dot_defer_run(scope)` pops and executes in LIFO order

---

## 7. Entry Point

```go
// Generate translates a typed Dot program into a complete C source string.
// info is the result of types.Check; prog is the AST root.
// Returns the C source code or an error if codegen fails.
func Generate(info *types.Info, prog *ast.Program) (string, error)
```

Located in `codegen/codegen.go`. Workflow:
1. Emit `#include "dot_runtime.h"` and `#include <stdint.h>` etc.
2. Emit forward declarations for all structs, enums, vtables.
3. Emit type definitions (structs, enums, vtables) in dependency order.
4. Emit function declarations (forward) then definitions.
5. Emit `main()` wrapping top-level statements.
6. Return the complete string.

---

## 8. Task List

| # | Task | Target file | Est. lines | Dependencies |
|---|------|-------------|------------|--------------|
| 5.1 | `codegen.go` — `Generate()` entry point, C header/footer, top-level driver | `codegen/codegen.go` | 120 | — |
| 5.2 | `types.go` — type mapping + name mangling | `codegen/types.go` | 180 | 5.1 |
| 5.3 | `builtins.go` — builtin function C mappings | `codegen/builtins.go` | 150 | 5.2 |
| 5.4 | `emit_arc.go` — retain/release/arena helpers | `codegen/emit_arc.go` | 200 | 5.2 |
| 5.5 | `emit_expr.go` — expression codegen (all 21 expr nodes) | `codegen/emit_expr.go` | 380 | 5.2, 5.3, 5.4 |
| 5.6 | `emit_stmt.go` — statement codegen (all stmt nodes) | `codegen/emit_stmt.go` | 350 | 5.2, 5.4, 5.5 |
| 5.7 | `emit_decl.go` — declaration codegen + monomorphisation | `codegen/emit_decl.go` | 380 | 5.2, 5.3, 5.5, 5.6 |
| 5.8 | `codegen_test.go` — golden tests | `codegen/codegen_test.go` | 250 | 5.1–5.7 |
| 5.9 | Wire `dot build` and `dot run` into `main.go` | `main.go`, `cmd/build.go`, `cmd/run.go` | 80 | 5.1–5.8 |

**Total estimate**: ~2090 lines across all Phase 5 files.

### Execution order

```
5.1 -> 5.2 -> 5.3 -> 5.4 -> 5.5 -> 5.6 -> 5.7 -> 5.8 -> 5.9
```

Steps 5.3 and 5.4 can be done in parallel after 5.2.

---

## 9. Interaction with Phase 4 (types.Info)

| Info field | Codegen use |
|------------|-------------|
| `Types[expr]` | Know the Dot type of every expression -> pick C type |
| `Defs[node]` | Map declaring node -> Symbol (for name mangling) |
| `Uses[ident]` | Resolve identifier reference -> Symbol |
| `Selections[fieldExpr]` | Field access resolution (field path, method, variant) |
| `Calls[callExpr]` | Argument order, defaults, variadic packing, builtin ID |
| `Instances[expr]` | Generic instantiation site -> mangled name + type args |
| `InstanceList` | Iterate all monomorphisations to emit |
| `Heap[expr]` | Whether an expression produces a heap value (ARC decisions) |
| `Arena[node]` | Whether inside `@perf` block (skip ARC, use arena) |
| `Terminates[stmt]` | Whether a branch falls through (match exhaustiveness) |

---

## 10. Known Limitations (v0.1)

- No closures with captures (lambda codegen is simplified; full closure support in Phase 6)
- No `dyn` trait object dispatch beyond basic fat-pointer (full vtable in Phase 6)
- No RTTI for `match` on types (emit panic)
- No string interpolation codegen (parser handles interpolation; codegen emits `dot_string_interp`)
- No multi-file compilation (single-file only until Phase 7)
- No cross-function escape analysis (all structs are heap-allocated by default)
- No `async`/`await` real implementation (emit stubs calling runtime)
- No `spawn` real threading (emit `dot_spawn` runtime stub)
