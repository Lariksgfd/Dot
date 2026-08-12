>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
>>> HANDOFF: START HERE IN NEW CHAT <<<
>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

## EXACT NEXT ACTIONS FOR NEW CHAT
1. **Finish LLVM method calls and standard library linkage.**
2. **Self-Hosting:** Implement `compiler/parser.dot` and `compiler/sema.dot`.
3. **Borrow Checker:** Implement advanced ARC / Borrow Checker rules.

>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

# ACCOMPLISHED IN PREVIOUS MEGA-SESSION
- **Phase 1 & 2 Complete:** AST comments, Imports, DotPM, Async Event Loop, TCP/HTTP, Crypto, Regex, Strings, Sync.
- **Phase 3 (LLVM Backend):** Heavily fleshed out (Structs, Enums, Match, Generics, Arrays, Pointers, ARC generation).
- **Phase 4 (LSP Server):** Foundation built.
- **Phase 6 (Self-Hosting Started):** `compiler/ast.dot` and `compiler/lexer.dot` are written and pass `dot check`.

---

# Historical Progress
1. Implemented multi-file imports with circular dependency checks.
2. Completed attaching buffered comments to AST nodes and fixed dot fmt AST dumping.
3. Wired up the LLVM IR generator to the dot CLI, including the -llvm flag and clang integration.
4. Fleshed out the LLVM IR Emitter, implementing basic math operations (+, -, *, /), integer literals, and variable declarations.
5. Exposed the C event loop to Dot language via stdlib/async.dot and runtime bindings.
6. Implemented TCP sockets support in runtime (net.h/c) and stdlib (net.dot).
7. Implemented simple HTTP client and server struct in stdlib/http.dot
8. Implemented Regular Expressions (std.regex) for Phase 2 with POSIX C wrappers and dot struct.
9. Added Control Flow (IfExpr, ForStmt, BlockExpr) to the LLVM IR Emitter.
10. Added LLVM IR emission for function calls (CallExpr) and mapped function arguments.
11. Implemented LLVM IR emission for struct declarations (StructDecl), struct instantiations (StructLit), and field access (FieldExpr using GetElementPtr).
