# Dot

Dot is a statically typed, compiled programming language. The compiler is written in Go and emits C99, which is then built with a standard C toolchain (gcc or clang). C acts as a portable low-level backend, so any platform with a C compiler is a potential target. A direct LLVM backend exists as a prototype.

## Status

Early development, but the core is real and works end to end: source code goes through a lexer, a parser, a type checker and a C code generator, and the resulting C is compiled into a native executable. The pipeline is covered by unit and end-to-end tests.

A self-hosting bootstrap is in progress: the lexer, parser and type checker already exist as Dot sources under `stdlib/` and pass the full test suite when compiled by the Go frontend. The code generator port is nearly complete.

Known rough edges: a few code generator bugs remain (documented internally), `dot fmt` is a stub and must not be used, the LSP is a prototype, and the standard library is a mix of complete modules and thin C bindings. See Roadmap below.

## Features

- Static typing with type inference
- Enums with payloads and pattern matching
- Structs, traits and impls (methods)
- Generics (functions, enums)
- First-class functions, closures, higher-order functions
- Automatic reference counting (ARC) — no manual memory management
- Asynchronous runtime with coroutines (`std.async`)
- OS threads and channels (`std.sync`, `std.chan`)
- Interop with C via `@extern("C")`
- Standard library: strings, collections, JSON, math, datetime, filesystem, and more

## Installation

Requirements:

- **Go 1.26+** (to build the compiler)
- **gcc** or **clang** on PATH (the compiler emits C and invokes it)

Windows, Linux and macOS are supported.

```bash
git clone https://github.com/Lariksgfd/Dot.git
cd Dot
go build -o dot .
```

Optionally add the binary to your PATH so you can run `dot` from anywhere.

## Quick start

Create `hello.dot`:

```dot
fn main() {
    print("Hello, world!")
}
```

Compile and run it:

```bash
dot run hello.dot
```

Or compile to a standalone executable:

```bash
dot build hello.dot
```

## Command-line interface

```
dot run <file.dot>       # compile and run a program
dot build <file.dot>     # compile to an executable
dot test                 # run dot tests
dot check <file.dot>     # type check only, no codegen
dot lex  <file.dot>      # dump the token stream
dot parse <file.dot>     # dump the AST
dot fmt  <file.dot>      # format source (stub, not yet safe to use)
dot init <name>          # create a new project skeleton
dot fetch                # fetch dependencies (package manager prototype)
dot install              # install dependencies (package manager prototype)
dot analyz               # language server (prototype)
```

## Language tour

### Variables and type inference

```dot
fn main() {
    a = 10          // inferred as int
    b = 3.14        // inferred as float
    name = "Dot"    // inferred as string
    flag = true     // inferred as bool
}
```

Types are optional on declarations; use `-> T` to declare return types explicitly.

### Functions and control flow

```dot
fn is_prime(n int) -> int {
    if n < 2 { return 0 }
    d = 2
    for d * d <= n {
        if n % d == 0 { return 0 }
        d = d + 1
    }
    return 1
}

fn fibonacci(n int) -> int {
    if n <= 1 { return n }
    f0 = 0
    f1 = 1
    cnt = 2
    for cnt <= n {
        f2 = f0 + f1
        f0 = f1
        f1 = f2
        cnt = cnt + 1
    }
    return f1
}

fn main() {
    print(is_prime(97))
    print(fibonacci(10))
}
```

`if`, `for` and `while` loops, `break`, `continue` and `return` behave as expected.

### Structs and methods

```dot
struct Point {
    x float
    y float
}

impl Point {
    fn new(x float, y float) -> Point {
        return Point { x: x, y: y }
    }

    fn x_val(self) -> float {
        return self.x
    }
}

fn main() {
    p = Point.new(3.0, 4.0)
    print(p.x_val())
}
```

### Enums with payloads and pattern matching

```dot
enum Option {
    None
    Some(x int)
}

fn unwrap_or(o Option, dflt int) -> int {
    return match o {
        Option.Some(v) => v
        Option.None => dflt
    }
}

fn main() {
    print(unwrap_or(Some(42), 0))   // 42
    print(unwrap_or(None, 0))       // 0
}
```

Match supports payload destructuring, guards, block-bodied arms and the `_` catch-all pattern:

```dot
r = match word {
    "hello" => "H"
    "world" => "W"
    _ => "?"
}
```

### Generics

```dot
fn identity[T](x T) -> T {
    return x
}

fn main() {
    print(identity(5))
    print(identity("text"))
}
```

### Closures and higher-order functions

```dot
fn apply(f fn(int) -> int, v int) -> int {
    return f(v)
}

fn main() {
    base = 10
    add = fn(x int) -> int { x + base }
    print(add(1))          // 11
    print(apply(add, 5))   // 15
}
```

Closures capture variables by value with automatic reference counting.

### Collections

Slices, arrays, maps and strings are built in:

```dot
import std.map

fn main() {
    nums = [1, 2, 3]
    nums.push(4)
    print(nums.len)          // 4
    print(nums[0])           // 1

    m = HashMap.new()
    m.put("hello", "world")
    print(m.get("hello"))    // world
    if m.contains("hello") {
        print("found")
    }
}
```

### Concurrency

Coroutine-based async runtime:

```dot
import std.async
import std.io

fn task() {
    print_str("task running\n")
    async_yield()
    print_str("task finished\n")
}

fn main() {
    async_spawn(task)
    async_spawn(task)
}
```

OS threads and channels are available through `std.sync` and `std.chan`.

### C interop

```dot
@extern("C")
fn sha256(input string) -> string

fn main() {
    print(sha256("dot"))
}
```

### Memory management

All heap values are managed with automatic reference counting. You never free memory manually; the compiler inserts retain/release pairs and moves ownership where possible. An optional borrow checker is planned for zero-cost borrowing in hot paths.

## Standard library

| Module | Contents | Status |
|--------|----------|--------|
| `std.strings` | uppercase/lowercase, split, join, search | complete |
| `std.json` | JSON parse/serialize | complete |
| `std.map` | HashMap | complete |
| `std.math` | math functions | complete |
| `std.datetime` | date/time helpers | complete |
| `std.testing` | test assertions | complete |
| `std.io`, `std.os`, `std.fs` | console, environment, files | thin C bindings |
| `std.net` | TCP/UDP sockets | thin C bindings |
| `std.time` | timers, sleeps | thin C bindings |
| `std.sync`, `std.chan` | threads, mutexes, channels | thin C bindings |
| `std.async` | coroutine runtime | thin C bindings |
| `std.crypto` | hashes (sha256) | thin C bindings |
| `std.regex` | regular expressions | thin C bindings |
| `std.collections`, `std.http`, `std.process`, `std.fmt` | — | stubs |

## Self-hosting

The standard library includes a second implementation of the compiler written in Dot itself: `stdlib/lexer.dot`, `stdlib/parser.dot`, `stdlib/checker.dot` and `stdlib/codegen*.dot`. The self-hosted lexer, parser and checker are complete and covered by smoke tests; the self-hosted code generator is nearly complete. When the bootstrap finishes, the compiler will be able to compile itself.

## Repository layout

```
lexer/          # tokenizer
parser/         # parser, AST
types/          # type checker, inference, scopes
codegen/        # C backend (LLVM prototype in codegen/llvm/)
runtime/        # C runtime: ARC, strings, slices, maps, async, io
stdlib/         # standard library in Dot, incl. the self-hosted compiler
ast/            # AST node definitions and printer
dotpm/          # package manager prototype
lsp/            # language server prototype
testdata/       # end-to-end test programs
```

## Roadmap

1. Finish the self-hosted code generator and bootstrap the compiler
2. Fix the remaining code generator bugs
3. Real `dot fmt` formatter
4. Module namespaces and visibility modifiers
5. Expand `std.net`, `std.crypto`, `std.regex` to full implementations
6. Fix the LSP, add editor support
7. Stabilize the LLVM backend (SIMD, optimizations)
8. WASM target and JIT

## License

MIT. See LICENSE.
