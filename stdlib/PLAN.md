# Phase 7: Standard Library & Phase 8: CLI & Tooling — Plan

---

## Phase 7: Standard Library

### 0. Pre-Analysis: What's Achievable in v0.1

Given the current C runtime (ARC, String, Slice, Map, Arena, Iter, Channel stub, Future stub, Weak, Dyn, Builtins), here's what each SPEC §16 package needs:

| Package | Pure Dot? | Needs C runtime? | v0.1 feasible? |
|---------|-----------|-----------------|----------------|
| `std/io` | print/input exist | fopen, fread, fwrite, fclose, fprintf | YES (C runtime exts) |
| `std/fs` | No | fopen, fstat, opendir, mkdir, remove, rename | YES (C runtime exts) |
| `std/os` | Minimal | getenv, setenv, getcwd, exit, args | YES (C runtime exts) |
| `std/fmt` | YES — pure string manipulation | No | YES |
| `std/math` | pow exists | sqrt, sin, cos, tan, abs, floor, ceil, round, log | YES (C runtime exts via math.h) |
| `std/time` | No | time(), localtime, strftime, sleep | YES (C runtime exts) |
| `std/collections` | YES — Set, Deque, Heap | No | YES |
| `std/json` | Tokenizer/parser in Dot | No | YES (v0.1 basic: decode only) |
| `std/testing` | YES — assert exists | No | YES |
| `std/net` | No | Full socket API (socket, bind, connect) | DEFERRED |
| `std/http` | No | Depends on net + sockets | DEFERRED |
| `std/sync` | No | Threads, mutex, atomics | DEFERRED |
| `std/crypto` | No | Big integer, C libraries | DEFERRED |
| `std/regex` | No | POSIX regex or PCRE | DEFERRED |

### 0.1 Architecture: Two Tiers

**Tier 1 — Pure Dot packages**: `std/fmt`, `std/collections`, `std/json`, `std/testing`
- Written entirely in Dot
- Call runtime builtins (print, len, assert)
- No C code needed

**Tier 2 — C-backed packages**: `std/io`, `std/fs`, `std/os`, `std/math`, `std/time`
- `.dot` files declare `@extern("C")` function signatures (types only, no bodies)
- C runtime `.c` files provide the real implementation
- Compiler links C runtime files automatically when import is detected

**Key design constraint**: `@extern("C")` annotation and `import` resolution are NOT yet implemented. They MUST be part of Phase 7.

### 0.2 Multi-file Import Strategy (v0.1 Minimal)

For v0.1, imports are path-based and stdlib-only:
- `import std.io` → resolves to `<compiler_root>/stdlib/io.dot`
- Stdlib `.dot` files are ONLY type declarations (no function bodies, all `@extern("C")`)
- No user-to-user imports (deferred to post-v0.1)
- No circular imports possible (stdlib is a flat namespace)
- Compiler auto-links corresponding C runtime `.c` files when stdlib import detected

Compiler changes for imports:
1. Parser: parse `import` statements → `*ast.ImportDecl` node
2. Checker: resolve path → parse imported file → merge declarations into scope → mark `@extern("C")` functions
3. Codegen: skip body emission for `@extern("C")` fns, add `#include` for runtime headers, collect C source files to link

---

## 1. File Breakdown

### 1.1 New C Runtime Files (Tier 2 backing)

| File | Purpose | Est. lines |
|------|---------|------------|
| `runtime/io.h` | File I/O: DotFile, dot_file_open/read/write/close/read_all/write_all, dot_print_str | ~60 |
| `runtime/io.c` | File I/O implementation via stdio.h | ~180 |
| `runtime/math.h` | Math wrappers: dot_math_sqrt/sin/cos/tan/abs/min/max/floor/ceil/round/log | ~50 |
| `runtime/math.c` | Math implementation via math.h | ~120 |
| `runtime/time.h` | Time: dot_time_now/dot_time_sleep, DotTimestamp struct | ~40 |
| `runtime/time.c` | Time implementation via time.h + unistd.h (sleep) | ~100 |
| `runtime/os.h` | OS: dot_os_getenv/dot_os_setenv/dot_os_getcwd/dot_os_exit/dot_os_args | ~50 |
| `runtime/os.c` | OS implementation via stdlib.h + Windows APIs | ~120 |

Subtotal runtime: ~720 lines across 8 files.

### 1.2 Stdlib .dot Files (Tier 1 + Tier 2 declarations)

| File | Purpose | Tier | Est. lines |
|------|---------|------|------------|
| `stdlib/io.dot` | I/O: File type, open, read, write, close, read_all, write_all, stdin/stdout/stderr | 2 (C) | ~80 |
| `stdlib/fs.dot` | Filesystem: exists, is_dir, is_file, mkdir, remove, rename, read_dir, ext, basename | 2 (C) | ~100 |
| `stdlib/os.dot` | OS: getenv, setenv, getcwd, exit, args, platform | 2 (C) | ~70 |
| `stdlib/math.dot` | Math: sqrt, sin, cos, tan, abs, min, max, floor, ceil, round, log, log2, log10, PI, E, deg→rad, rad→deg | 2 (C) | ~90 |
| `stdlib/time.dot` | Time: Timestamp, now, sleep, format, since, until | 2 (C) | ~80 |
| `stdlib/fmt.dot` | Formatting: format, pad_left, pad_right, format_float, hex, binary, octal | 1 (pure Dot) | ~120 |
| `stdlib/testing.dot` | Testing: assert_eq, assert_ne, assert_true, assert_false, assert_none, assert_some, assert_err, assert_ok | 1 (pure Dot) | ~100 |
| `stdlib/collections.dot` | Set[T], Deque[T] — pure Dot implementations | 1 (pure Dot) | ~200 |
| `stdlib/json.dot` | JSON: encode (serialize), decode (parse) — basic v0.1 subset (no custom types) | 1 (pure Dot) | ~250 |
| `stdlib/std.dot` | Re-exports: `pub use std.io; pub use std.fmt; ...` — convenience prelude | 1 | ~30 |

Subtotal stdlib: ~1120 lines across 10 files.

### 1.3 Compiler Changes (Go)

| File | Purpose | Est. lines |
|------|---------|------------|
| `codegen/emit_extern.go` | NEW: `@extern("C")` handling — emit `extern` decl, skip body, track needed runtime includes | ~80 |
| `types/import.go` | NEW: Import resolution — parse `import` decl, resolve stdlib path, parse imported file, collect `@extern` decls, merge into scope | ~250 |
| `ast/decl.go` | MODIFY: Add `ImportDecl` node type | ~30 |
| `parser/decl.go` | MODIFY: Parse `import` statement | ~40 |
| `parser/decl_type.go` | MODIFY: Parse `@extern("C")` annotation on function | ~50 |
| `ast/decl.go` | MODIFY: Add `Annotations` field to `FnDecl` | ~15 |
| `codegen/emit_decl.go` | MODIFY: Skip body for `@extern("C")` functions | ~25 |
| `codegen/codegen.go` | MODIFY: Accept list of imported stdlib modules, emit `#include` for them, track C source files to link | ~60 |
| `main.go` | MODIFY: `dot build`/`dot run` — resolve imports, auto-link stdlib C files, pass stdlib path to codegen | ~70 |

Subtotal compiler: ~620 lines across 7 new/modified files.

### 1.4 Total Phase 7

| Category | Files | Lines |
|----------|-------|-------|
| C runtime (new) | 8 | ~720 |
| .dot stdlib | 10 | ~1120 |
| Go compiler | 7–10 | ~620 |
| **Total** | **25-28** | **~2460** |

---

## 2. Task List — Phase 7

### Block A: `@extern("C")` annotation support (foundation)

| # | Task | Files | Lines | Deps |
|---|------|-------|-------|------|
| 7.1 | Add `ImportDecl` to AST: `ast.ImportDecl { Path string; Alias string; Annotations []AnnotationDecl }` | `ast/decl.go` | ~30 | — |
| 7.2 | Add annotations to `FnDecl`: `Annotations []AnnotationDecl` field. Add `AnnotationDecl` node type to AST. | `ast/decl.go` | ~25 | — |
| 7.3 | Parser: parse `import "path"` and `import "path" as alias` syntax | `parser/decl.go` | ~40 | 7.1 |
| 7.4 | Parser: parse `@extern("C")`, `@inline`, `@deprecated`, `@test` annotations before `fn` declarations | `parser/decl_type.go` | ~50 | 7.2 |
| 7.5 | Codegen: `emit_extern.go` — when `FnDecl` has `@extern("C")`, emit `extern` forward-decl only, no body. Track which runtime headers to include. | `codegen/emit_extern.go` | ~80 | 7.2, 7.4 |
| 7.6 | Codegen: modify `emitFuncDefs` to skip functions with `@extern("C")` (body is in C runtime) | `codegen/emit_decl.go` | ~25 | 7.5 |

### Block B: Import resolution (plug-in stdlib)

| # | Task | Files | Lines | Deps |
|---|------|-------|-------|------|
| 7.7 | Create `types/import.go`: `resolveImport(importDecl, currentDir, stdlibDir)` — resolves path, parses file, collects `@extern` decls, merges into scope. Returns list of C runtime modules needed. | `types/import.go` | ~250 | 7.1, 7.3, 7.4 |
| 7.8 | Modify `types.Check()` or add `CheckMulti()` to accept import list. For v0.1: modify Check to process `ImportDecl` nodes (parse imported file before pass 1). | `types/checker.go` | ~40 | 7.7 |
| 7.9 | Modify `types/collect.go`: process imported symbols — add them to scope as forward-declared functions (no body checking, already type-annotated). | `types/collect.go` | ~40 | 7.7 |

### Block C: C Runtime Extensions

| # | Task | Files | Lines | Deps |
|---|------|-------|-------|------|
| 7.10 | Implement `runtime/io.h` + `runtime/io.c`: `DotFile* dot_file_open, dot_file_close, dot_file_read, dot_file_write, dot_file_read_all, dot_file_write_all`. Also `void dot_print_str(DotString*)` — raw string print (no interpolation). | `runtime/io.h`, `runtime/io.c` | ~240 | — |
| 7.11 | Implement `runtime/math.h` + `runtime/math.c`: wrap libm functions in Dot-typed API (`int64_t`, `double`). Functions: sqrt, sin, cos, tan, abs, fabs, min, max, floor, ceil, round, log, log2, log10, pow (already have dot_pow_*), hypot. | `runtime/math.h`, `runtime/math.c` | ~170 | — |
| 7.12 | Implement `runtime/time.h` + `runtime/time.c`: `DotTimestamp` struct (int64_t seconds, int32_t nanos). `dot_time_now()`, `dot_time_sleep(int64_t ms)`, `dot_time_since(t)`, `dot_time_format(t, fmt)`. | `runtime/time.h`, `runtime/time.c` | ~140 | — |
| 7.13 | Implement `runtime/os.h` + `runtime/os.c`: `dot_os_getenv(name)→string`, `dot_os_setenv(name,val)`, `dot_os_getcwd()→string`, `dot_os_exit(code)`, `dot_os_args()→[]string`, `dot_os_platform()→string`. | `runtime/os.h`, `runtime/os.c` | ~170 | — |
| 7.14 | Update `runtime/dot_runtime.h` to `#include` all new headers (io.h, math.h, time.h, os.h). | `runtime/dot_runtime.h` | ~10 | 7.10–7.13 |

### Block D: Stdlib .dot Files

| # | Task | Files | Lines | Deps |
|---|------|-------|-------|------|
| 7.15 | Write `stdlib/io.dot`: declare `@extern("C") fn`s for all io.c functions. Add Dot wrapper types: `Stdout`, `Stdin`, `Stderr` (pre-opened File instances). | `stdlib/io.dot` | ~80 | 7.10 |
| 7.16 | Write `stdlib/fs.dot`: declare fs operations. Define `FileInfo` struct, `DirEntry` struct. | `stdlib/fs.dot` | ~100 | 7.10 |
| 7.17 | Write `stdlib/os.dot`: declare OS operations. | `stdlib/os.dot` | ~70 | 7.13 |
| 7.18 | Write `stdlib/math.dot`: declare math functions. Add constants (PI, E, TAU). Add convenience fns: deg_to_rad, rad_to_deg. | `stdlib/math.dot` | ~90 | 7.11 |
| 7.19 | Write `stdlib/time.dot`: declare time operations. Add Duration helpers (ms, sec, min, hour). | `stdlib/time.dot` | ~80 | 7.12 |
| 7.20 | Write `stdlib/fmt.dot`: PURE DOT. String formatting: `format(template, args...)`, `pad_left(s, width, pad_char)`, `pad_right`, `format_int(n, base)`, `format_float(f, precision)`, `to_upper`, `to_lower`, `trim`, `split`, `join`, `contains`, `starts_with`, `ends_with`, `replace`. | `stdlib/fmt.dot` | ~120 | 7.5 (no C dep, uses builtins) |
| 7.21 | Write `stdlib/testing.dot`: PURE DOT. `assert_eq(a,b)`, `assert_ne(a,b)`, `assert_true(cond)`, `assert_false(cond)`, `assert_none(opt)`, `assert_some(opt)`, `assert_err(res)`, `assert_ok(res)`. Each calls `assert()` with a descriptive message. | `stdlib/testing.dot` | ~100 | 7.5 |
| 7.22 | Write `stdlib/collections.dot`: PURE DOT. Generic `Set[T]` (backed by `map[T]bool`), `Deque[T]` (backed by `[]T` with head/tail). Operations: new, add, remove, contains, len, push_front, push_back, pop_front, pop_back. | `stdlib/collections.dot` | ~200 | 7.5 |
| 7.23 | Write `stdlib/json.dot`: PURE DOT. JSON decoder: `decode(s string)→JsonValue`. JSON encoder: `encode(v JsonValue)→string`. `JsonValue` enum: Null, Bool(val), Number(val float), String(val string), Array(vals []JsonValue), Object(vals map[string]JsonValue). Parser with tokenizer. | `stdlib/json.dot` | ~250 | 7.5, 7.20 |
| 7.24 | Write `stdlib/std.dot`: convenience re-export module — re-exports commonly used symbols from io, fmt, math, collections, testing. | `stdlib/std.dot` | ~30 | 7.15–7.23 |
| 7.25 | Write `stdlib/PLAN.md` (this file) — captured during architect phase. | `stdlib/PLAN.md` | ~300 | all above |

### Block E: Integration (wiring)

| # | Task | Files | Lines | Deps |
|---|------|-------|-------|------|
| 7.26 | Modify `codegen/codegen.go`: `Generate()` accepts a list of imported stdlib module names. Emits `#include "io.h"`, `#include "math.h"` etc. for each imported module. Returns the C source AND the list of C runtime files to compile. | `codegen/codegen.go` | ~60 | 7.5, 7.9 |
| 7.27 | Modify `main.go`: build/run commands resolve imports, find stdlib directory, pass to checker → codegen. Auto-link all stdlib C files when building (gcc gets `runtime/*.c` + stdlib-relevant runtime files). | `main.go` | ~70 | 7.26 |
| 7.28 | End-to-end test: write `testdata/hello_stdlib.dot` that imports std.io and std.fmt, prints formatted output. Verify build + run. | `testdata/hello_stdlib.dot` | ~30 | 7.27 |

---

## 3. Task Execution Order (Phase 7)

```
Block A (foundation): 7.1 → 7.2 → 7.3, 7.4 (parallel) → 7.5 → 7.6
Block B (import): 7.7 → 7.8, 7.9 (parallel)
Block C (runtime): 7.10, 7.11, 7.12, 7.13 (all parallel) → 7.14
Block D (stdlib): 7.15–7.24 (all parallel, each depends only on its C runtime counterpart + 7.5)
Block E (integration): 7.25 → 7.26 → 7.27 → 7.28

Critical path: 7.1 → 7.2 → 7.3/7.4 → 7.5 → 7.6 → 7.7 → 7.8/7.9 → 7.26 → 7.27 → 7.28
Runtime + stdlib can proceed in parallel with compiler work.
Total: 28 tasks.
```

---

## 4. Design Decisions (continuing D-series)

- **D86**: `@extern("C")` functions are declared with `extern` in C output and have NO body emitted. The corresponding C runtime function must exist in the linked .c files. The function's C name is the mangled Dot name (e.g., `Dot_io_File_open`).

- **D87**: Stdlib imports are resolved relative to a `STDLIB_DIR` embedded at build time or derived from the compiler binary path. In development, it's the `stdlib/` directory next to the project root.

- **D88**: Imported stdlib `.dot` files only contain function DECLARATIONS with full type annotations. No bodies. No inference needed. This makes import resolution a lightweight pass (parse + collect decls, skip checkBodies).

- **D89**: The C runtime files linked at compile time are determined by which stdlib packages are imported. Unused stdlib C files are NOT compiled (to avoid pulling in unnecessary symbols). The mapping is: `import std.io` → link `runtime/io.c`, `import std.math` → link `runtime/math.c`, etc.

- **D90**: Stdlib package names are flat: `std.io`, `std.fmt`, NOT hierarchical beyond one level. This keeps resolution simple for v0.1.

- **D91**: For Tier 1 (pure Dot) stdlib packages, the .dot files contain both declarations AND bodies. These bodies generate C code just like user code. When imported, they are merged into the compilation unit and codegen processes them normally.

- **D92**: The `json.dot` package defines a `JsonValue` enum internal to itself. User code accesses JSON values through this enum. No RTTI needed — the user matches on `JsonValue.Null`, `JsonValue.String(s)`, etc.

- **D93**: `DotFile` in the C runtime is a struct wrapping `FILE*` with refcount. `dot_file_open` allocates a `DotFile`, `dot_file_close` releases it. Stdio handles (stdin/stdout/stderr) have special refcount handling (never freed).

---

## 5. What's Explicitly DEFERRED Beyond v0.1

| Item | Reason |
|------|--------|
| `std/net` (TCP/UDP sockets) | Requires platform-specific socket API wrappers. Too much surface area for v0.1. |
| `std/http` (client + server) | Depends on `std/net`. |
| `std/crypto` (MD5, SHA, AES) | Requires big integer math and/or external C crypto library. |
| `std/regex` | Requires POSIX regex or PCRE integration. |
| `std/sync` (Mutex, Atomic, Semaphore) | Requires real threading primitives. Channel/Future are stub single-threaded for now. |
| User-to-user multi-file imports | Only stdlib imports work. User code is single-file with `import std.*` support. |
| Package-level visibility (`_private`) | All declarations are public. No module system beyond file level. |
| `@deprecated` enforcement | Parser parses it, checker ignores it for v0.1 (warnings only emit in v0.2). |
| `@inline` hint to C compiler | Parsed but ignored. GCC decides inlining on its own. |
| String interpolation in `print` | `print("Hello {name}")` only works for the builtin `print`/`println`/`eprint`. `std.fmt.format` uses a different syntax or positional args. |
| RTTI-based `match` on types | No type info at runtime. `match value { int n => ..., string s => ... }` emits `dot_panic("type match not in v0.1")`. |
| Real `spawn` threading | `spawn { }` runs synchronously in v0.1. `DotFuture` resolve is immediate. |
| Real `async/await` | `await` just calls the function synchronously. No event loop. |
| `dot get` (package manager) | No remote package fetching. |

---

## Phase 8: CLI & Tooling

### 0. Current State

All CLI commands are in `main.go` (232 lines). The switch statement dispatches `lex`, `parse`, `check`, `build`, `run`. There is no `cmd/` package yet.

ARCHITECTURE.md shows expected structure:
```
cmd/
    build.go    # dot build
    run.go      # dot run
    test.go     # dot test
    fmt.go      # dot fmt
    check.go    # dot check
    init.go     # dot init
```

### 1. File Breakdown — Phase 8

| File | Purpose | Est. lines |
|------|---------|------------|
| `main.go` | MODIFY: Trim to ~30 lines — just CLI dispatch to cmd/ package | ~30 |
| `cmd/cmd.go` | NEW: Common helpers for all commands: `readSource`, `report`, shared pipeline (parse→check→codegen→gcc), `findStdlibDir` | ~120 |
| `cmd/build.go` | NEW: `dot build <file> [-o output]` — moved from main.go, enhanced with stdlib linking | ~80 |
| `cmd/run.go` | NEW: `dot run <file>` — moved from main.go, enhanced with stdlib linking | ~60 |
| `cmd/check.go` | NEW: `dot check <file>` — moved from main.go | ~40 |
| `cmd/lex.go` | NEW: `dot lex <file>` — moved from main.go | ~30 |
| `cmd/parse.go` | NEW: `dot parse <file>` — moved from main.go | ~30 |
| `cmd/test.go` | NEW: `dot test [pattern]` — find @test functions, build test binary, execute | ~180 |
| `cmd/fmt.go` | NEW: `dot fmt <file>` — read, parse, ast.Print, write back | ~80 |
| `cmd/init.go` | NEW: `dot init <name>` — create project skeleton (main.dot, .gitignore, optional stdlib symlink) | ~100 |

Subtotal Phase 8: ~720 lines across 9 files (plus main.go rewrite).

### 2. Task List — Phase 8

| # | Task | Files | Lines | Deps |
|---|------|-------|-------|------|
| 8.1 | Create `cmd/cmd.go`: shared pipeline function `compileFile(path, output, mode)` that does parse→check→codegen→gcc. Includes `findStdlibDir()` returning the stdlib directory path relative to the compiler binary. | `cmd/cmd.go` | ~120 | Phase 7 complete |
| 8.2 | Create `cmd/lex.go`: moved from main.go `lexFile()`. | `cmd/lex.go` | ~30 | 8.1 |
| 8.3 | Create `cmd/parse.go`: moved from main.go `parseFile()`. | `cmd/parse.go` | ~30 | 8.1 |
| 8.4 | Create `cmd/check.go`: moved from main.go `checkFile()`. Enhanced to report import errors. | `cmd/check.go` | ~40 | 8.1 |
| 8.5 | Create `cmd/build.go`: moved from main.go `runBuild()`. Enhanced: resolve imports, auto-link stdlib C runtime .c files. | `cmd/build.go` | ~80 | 8.1, Phase 7 |
| 8.6 | Create `cmd/run.go`: moved from main.go `runRun()`. Enhanced same as build. | `cmd/run.go` | ~60 | 8.1, Phase 7 |
| 8.7 | Create `cmd/test.go`: `dot test` command. Scans AST for `@test`-annotated functions. Generates a C test runner that calls each @test fn. If any test calls `assert()` and it fails → `dot_panic()` exits non-zero. Test runner: count tests, run each, report pass/fail, exit 0 if all pass. | `cmd/test.go` | ~180 | 8.1, 7.4 |
| 8.8 | Create `cmd/fmt.go`: `dot fmt <file>`. Read file → parse → `ast.Print(prog)` → write back to same file. Error if parse fails. | `cmd/fmt.go` | ~80 | 8.1, parser, ast printer |
| 8.9 | Create `cmd/init.go`: `dot init <name>`. Creates directory `<name>/`, writes `main.dot` (skeleton with `fn main() { print("Hello from {name}!") }`), writes `.gitignore` (`*.exe`, `*.o`, `.dot_build/`). Optionally copies or symlinks `runtime/` into project dir. | `cmd/init.go` | ~100 | 8.1 |
| 8.10 | Rewrite `main.go`: trim to ~30 lines. Dispatch all commands to `cmd/` package. Usage help updated. | `main.go` | ~30 | 8.2–8.9 |

### 3. `dot test` Detailed Design (task 8.7)

```
dot test [file.dot]           # test specific file
dot test                      # test main.dot or all .dot files
dot test --verbose            # print each test name as it runs
```

Algorithm:
1. Parse the target file(s)
2. Walk AST, collect all `FnDecl` nodes with `@test` annotation
3. If no @test functions found → "no tests found" exit 0
4. Generate a C test runner:
   ```c
   #include "dot_runtime.h"
   // Forward decls for all test fns
   void Dot_test_addition(void);
   void Dot_test_string_interpolation(void);
   
   int main(void) {
       int passed = 0, failed = 0;
       // For each test:
       printf("  test_addition... ");
       Dot_test_addition();
       printf("OK\n");
       passed++;
       // (if assert fails inside, dot_panic handles it)
       
       printf("\n%d passed, %d failed\n", passed, failed);
       dot_atexit_cleanup();
       return failed > 0 ? 1 : 0;
   }
   ```
5. Compile test binary (includes runtime + user code + test runner)
6. Execute test binary
7. Parse output, report summary

**Key issue**: `assert()` currently calls `dot_assert_fail()` which prints and exits. For testing, each test should run in isolation. Solution for v0.1: each @test function is compiled as a SEPARATE binary (one test per exe), or we accept "first assert failure terminates the entire test run". For v0.1 simplicity: **all tests run in one binary, first assert failure terminates with output showing which test failed**. The assert already prints the file:line. Good enough for v0.1.

Alternative (better): each test is called in a separate subprocess via `system()` or `fork()` → but that's complex. v0.1 = single process, terminate on first failure.

### 4. `dot fmt` Detailed Design (task 8.8)

```
dot fmt <file.dot>           # format file in-place
dot fmt --check <file.dot>   # check if formatted, exit 1 if not
```

Algorithm:
1. Read source file
2. Parse with `parser.ParseFile()` 
3. If parse fails → report errors, exit 1
4. Generate formatted output via `ast.Print(prog)` (already exists in ast/printer.go)
5. Write formatted output back to file

**Limitation v0.1**: `ast.Print` is a debug printer, not a production formatter. It doesn't:
- Preserve comments (parser doesn't store comments in AST yet)
- Handle line wrapping (long lines stay long)
- Normalize whitespace
But it produces valid, indented Dot code. Good enough for v0.1.

### 5. `dot init` Detailed Design (task 8.9)

```
dot init my_project
```

Creates:
```
my_project/
├── main.dot           # skeleton with fn main()
├── .gitignore         # *.exe, *.o, .dot_build/
└── runtime/           # copy of runtime/*.c + runtime/*.h (OR symlink)
```

The runtime copy ensures the project is self-contained and buildable without the compiler's runtime directory.

### 6. Design Decisions (Phase 8)

- **D94**: `cmd/cmd.go` provides a shared `compileFile()` that returns the executable path. All build/run/test commands use it. This avoids code duplication between build and run.

- **D95**: The stdlib directory is located relative to the compiler binary: `<binary_dir>/../stdlib/` (development layout) or `<binary_dir>/stdlib/` (release layout). `cmd/cmd.go` tries both.

- **D96**: `dot test` generates a single test binary that runs all @test functions sequentially. First assertion failure terminates the run. The assert message includes the test function name via `__func__` or the @test function's Dot name.

- **D97**: `dot fmt` uses the existing `ast.Print` printer. Comments are NOT preserved (the lexer/parser don't store comment nodes in the AST yet). This is a known limitation. A production formatter with comment preservation is deferred to v0.2.

- **D98**: `dot init` copies the runtime directory into the new project. This is ~26 files (~80KB). Acceptable for v0.1. A future version will use a shared runtime installation path.

- **D99**: The `cmd/` package is internal to the compiler (`package main` or `package cmd` imported by `main.go`). All commands share the same `main` package namespace (same binary).

---

## 6. Phase 8 Execution Order

```
8.1 → 8.2, 8.3, 8.4, 8.5, 8.6, 8.7, 8.8, 8.9 (all parallel after 8.1)
8.10 (after all others)

Total: 10 tasks.
```

Tasks 8.2 through 8.9 are independent of each other and can run in parallel once 8.1 is done.

---

## 7. Summary — Both Phases

| Metric | Phase 7 | Phase 8 | Total |
|--------|---------|---------|-------|
| New files | ~18 | ~8 | ~26 |
| Modified files | ~7 | 1 (main.go) | ~8 |
| C runtime files | 8 | 0 | 8 |
| .dot stdlib files | 10 | 0 | 10 |
| Go files | 7 | 10 | 17 |
| Total lines | ~2460 | ~720 | ~3180 |
| Tasks | 28 | 10 | 38 |

### Combined Execution Order

```
Phase 7 Block A (7.1–7.6)   → Foundation: @extern + annotations
Phase 7 Block B (7.7–7.9)   → Import resolution
Phase 7 Block C (7.10–7.14) → C runtime extensions (parallel with B)
Phase 7 Block D (7.15–7.24) → Stdlib .dot files
Phase 7 Block E (7.25–7.28) → Integration wiring
─── Phase 7 complete ───
Phase 8 Task 8.1            → Shared cmd helpers
Phase 8 Tasks 8.2–8.9       → All CLI commands (parallel)
Phase 8 Task 8.10           → main.go rewrite
─── Phase 8 complete ───
```

### Critical Path (fastest to working build)

```
7.1 → 7.2 → 7.4 → 7.5 → 7.6 → 7.10 → 7.15 → 7.27 → 7.28
(annotation parsing → extern codegen → io.c → io.dot → wiring → integration test)
```

This path gets a working `import std.io` + `print` program building in ~8 tasks. Then other stdlib packages and CLI commands fan out in parallel.

---

## 8. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Import resolution for pure Dot stdlib (tier 1) requires multi-file codegen — mixing C from two .dot files | High | Tier 1 stdlib `.dot` files are merged into a single `*ast.Program` before codegen. Codegen treats merged program identically to single-file program. |
| `std/json` decoder is ~250 lines of Dot — largest stdlib file. May hit language edge cases (generics in stdlib, no closures yet) | Medium | Keep JSON decoder simple: tokenize → recursive descent parse → build `JsonValue` tree. No extensible deserialization (no `from_json` trait). |
| Windows-specific C runtime calls (sleep, getcwd) differ from POSIX | Medium | Use `#ifdef _WIN32` in C runtime. `sleep` → `Sleep(ms)` on Windows, `usleep` on POSIX. Already have gcc/MinGW so Windows headers available. |
| `dot fmt` loses comments | Low (v0.1 known limitation) | Documented limitation. Comment nodes in AST are deferred to v0.2 parser update. |
| `dot test` termination on first failure is crude | Low | Acceptable for v0.1. Multi-test isolation via subprocess can be added later. |
