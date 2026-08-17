# Dot

Dot is a statically typed, compiled programming language. The compiler is written in Go and emits C, which is then built with a standard C toolchain (gcc/clang). An early-stage LLVM backend is included as well.

## Status

Early development. The core compiler pipeline is functional: lexer, parser, type checker, and C code generation are implemented and covered by end-to-end tests. A self-hosting bootstrap is in progress — the lexer, parser, and checker already exist as Dot sources in `stdlib/` and compile via the Go frontend.

## Features

- Static typing with type inference
- Enums with payloads, structs, traits and impls, generics
- Automatic reference counting (ARC) memory management
- First-class functions and closures
- `async`/`await` with a C runtime
- `spawn` for OS threads, channels for message passing
- Standard library covering collections, IO, networking, sync, and more

## Building

Requires Go 1.26+ and gcc (or clang) on PATH.

```
go build -o dot .
```

## Usage

```
dot run <file.dot>       # compile and run
dot build <file.dot>     # compile to an executable
dot test                 # run dot tests
dot check <file.dot>     # type check only
dot lex <file.dot>       # dump tokens
dot parse <file.dot>     # dump the AST
dot fmt <file.dot>       # format source
```

## Example

```
enum Shape {
    Circle(radius float)
    Rect(w float, h float)
}

fn area(s Shape) -> float {
    return match s {
        Shape.Circle(r) => 3.14159 * r * r
        Shape.Rect(w, h) => w * h
    }
}

fn main() {
    c = Shape.Circle(2.0)
    print(area(c))
}
```

## Repository layout

- `lexer/` — tokenizer
- `parser/` — AST and parser
- `types/` — type checker
- `codegen/` — C backend (and LLVM prototype under `codegen/llvm/`)
- `runtime/` — C runtime (ARC, strings, arrays, maps, async, io)
- `stdlib/` — Dot standard library, including the self-hosted compiler parts
- `dotpm/` — package manager prototype
- `lsp/` — language server prototype

## License

MIT. See LICENSE.
