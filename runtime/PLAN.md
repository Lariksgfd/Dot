# Phase 6: C Runtime Library — Plan

Implements the C functions and data structures that generated code calls.
Compiled alongside generated .c files to produce the final binary.

---

## 1. File Breakdown

| File | Purpose | Est. lines |
|------|---------|------------|
| `runtime/dot_runtime.h` | Master header: includes all runtime headers, common macros | ~60 |
| `runtime/dot_runtime.c` | Master init/cleanup: `dot_atexit_cleanup`, global state | ~80 |
| `runtime/arc.h` | ARC base: `DotRefcnt`, `dot_retain`, `dot_release` | ~50 |
| `runtime/arc.c` | ARC implementation | ~80 |
| `runtime/string.h` | `DotString` struct + string function declarations | ~40 |
| `runtime/string.c` | String implementation: from_lit, to_bytes, interp, concat | ~150 |
| `runtime/array.h` | `DotSlice` struct + slice function declarations | ~50 |
| `runtime/array.c` | Slice implementation: get, set, push, sub, pack | ~180 |
| `runtime/map.h` | `DotMap` struct + map function declarations | ~40 |
| `runtime/map.c` | Map implementation: new, get, set, remove, iter | ~200 |
| `runtime/arena.h` | `DotArena` struct + arena allocator declarations | ~30 |
| `runtime/arena.c` | Arena implementation: new, alloc, free | ~80 |
| `runtime/closure.h` | `DotClosure` struct + closure declarations | ~30 |
| `runtime/closure.c` | Closure implementation: new, call | ~60 |
| `runtime/iter.h` | `DotIter` struct + iterator declarations | ~30 |
| `runtime/iter.c` | Iterator implementation: collection, map, range | ~100 |
| `runtime/channel.h` | `DotChannel` struct + channel declarations | ~30 |
| `runtime/channel.c` | Channel implementation (stub for v0.1) | ~50 |
| `runtime/future.h` | `DotFuture` struct + future declarations | ~30 |
| `runtime/future.c` | Future/spawn implementation (stub for v0.1) | ~50 |
| `runtime/weak.h` | `DotWeak` struct + weak reference declarations | ~30 |
| `runtime/weak.c` | Weak reference implementation | ~60 |
| `runtime/dyn.h` | `DotDyn` struct + dynamic trait object declarations | ~30 |
| `runtime/dyn.c` | Dynamic dispatch helpers | ~50 |
| `runtime/builtins.h` | Builtin function declarations | ~40 |
| `runtime/builtins.c` | Builtin implementations: print, len, panic, assert, input | ~150 |

Total: ~1700 lines across 26 files.

---

## 2. Data Structures

### DotRefcnt (base for all refcounted types)
```c
typedef struct {
    int32_t refcount;
    int32_t size;  // for ARC-free deallocation
} DotRefcnt;
```

### DotString
```c
typedef struct {
    DotRefcnt rc;
    int64_t len;
    char data[];  // flexible array member
} DotString;
```

### DotSlice
```c
typedef struct {
    DotRefcnt rc;
    int64_t len;
    int64_t cap;
    DotAny* data;  // DotAny = void* with ARC
} DotSlice;
```

### DotMap
```c
typedef struct {
    DotRefcnt rc;
    int64_t len;
    int64_t cap;
    DotMapEntry* entries;
} DotMap;
typedef struct {
    uint64_t hash;
    DotAny key;
    DotAny value;
} DotMapEntry;
```

### DotArena
```c
typedef struct {
    char* buffer;
    int64_t len;
    int64_t cap;
} DotArena;
```

### DotClosure
```c
typedef struct {
    DotRefcnt rc;
    void* fn;
    DotAny* captures;
    int32_t ncaptures;
} DotClosure;
```

### DotIter
```c
typedef struct {
    int64_t state;  // index or position
    DotAny collection;
    int8_t has_key;
} DotIter;
```

### DotChannel
```c
typedef struct {
    DotRefcnt rc;
    DotSlice* buffer;
    int64_t head, tail, count, cap;
    // TODO: mutex/cond for blocking
} DotChannel;
```

### DotFuture
```c
typedef struct {
    DotRefcnt rc;
    DotAny result;
    int8_t ready;
    // TODO: thread/sync
} DotFuture;
```

### DotWeak
```c
typedef struct {
    DotRefcnt rc;
    DotRefcnt* target;
} DotWeak;
```

### DotDyn (trait object)
```c
typedef struct {
    void* data;
    void* vtable;
} DotDyn;
```

---

## 3. Function Inventory

### ARC (arc.h/arc.c)
| Function | Signature | Description |
|----------|-----------|-------------|
| `dot_retain` | `DotAny* dot_retain(DotRefcnt* obj)` | Increment refcount, return obj |
| `dot_release` | `void dot_release(DotRefcnt* obj)` | Decrement refcount, free if 0 |
| `dot_atexit_cleanup` | `void dot_atexit_cleanup(void)` | Global cleanup at program exit |

### String (string.h/string.c)
| Function | Signature |
|----------|-----------|
| `dot_string_from_lit` | `DotString* dot_string_from_lit(const char* ptr, int64_t len)` |
| `dot_string_from_bytes` | `DotString* dot_string_from_bytes(DotSlice* bytes)` |
| `dot_string_to_bytes` | `DotSlice* dot_string_to_bytes(DotString* s)` |

### Slice (array.h/array.c)
| Function | Signature |
|----------|-----------|
| `dot_slice_from_array` | `DotSlice* dot_slice_from_array(int64_t n, DotAny* arr)` |
| `dot_slice_get` | `DotAny dot_slice_get(DotSlice* s, int64_t i)` |
| `dot_slice_set` | `void dot_slice_set(DotSlice* s, int64_t i, DotAny v)` |
| `dot_slice_push` | `void dot_slice_push(DotSlice* s, DotAny v)` |
| `dot_slice_sub` | `DotSlice* dot_slice_sub(DotSlice* s, int64_t lo, int64_t hi, bool inclusive)` |
| `dot_slice_pack` | `DotSlice* dot_slice_pack(int64_t n, DotAny* args)` |
| `dot_slice_len` | `int64_t dot_slice_len(DotSlice* s)` |
| `dot_slice_cap` | `int64_t dot_slice_cap(DotSlice* s)` |

### Map (map.h/map.c)
| Function | Signature |
|----------|-----------|
| `dot_map_new` | `DotMap* dot_map_new(void)` |
| `dot_map_get` | `DotAny dot_map_get(DotMap* m, DotAny key)` |
| `dot_map_set` | `void dot_map_set(DotMap* m, DotAny key, DotAny value)` |
| `dot_map_remove` | `void dot_map_remove(DotMap* m, DotAny key)` |

### Arena (arena.h/arena.c)
| Function | Signature |
|----------|-----------|
| `dot_arena_new` | `DotArena* dot_arena_new(int64_t initial_size)` |
| `dot_arena_alloc` | `void* dot_arena_alloc(DotArena* a, int64_t size)` |
| `dot_arena_free` | `void dot_arena_free(DotArena* a)` |

### Closure (closure.h/closure.c)
| Function | Signature |
|----------|-----------|
| `dot_closure_new` | `DotClosure* dot_closure_new(void* fn, DotAny* captures, int32_t n)` |
| `dot_closure_call` | `DotAny dot_closure_call(DotClosure* c, DotAny* args)` |

### Iterator (iter.h/iter.c)
| Function | Signature |
|----------|-----------|
| `dot_iter` | `DotIter dot_iter(DotAny collection)` |
| `dot_iter_next` | `bool dot_iter_next(DotIter* it, DotAny* key, DotAny* val)` |

### Channel (channel.h/channel.c)
| Function | Signature |
|----------|-----------|
| `dot_channel_new` | `DotChannel* dot_channel_new(int64_t cap)` |
| `dot_channel_send` | `void dot_channel_send(DotChannel* ch, DotAny val)` |
| `dot_channel_recv` | `DotAny dot_channel_recv(DotChannel* ch)` |

### Future (future.h/future.c)
| Function | Signature |
|----------|-----------|
| `dot_future_get` | `DotAny dot_future_get(DotFuture* f)` |
| `dot_spawn` | `DotFuture* dot_spawn(void* fn, DotAny ctx)` |

### Builtins (builtins.h/builtins.c)
| Function | Signature |
|----------|-----------|
| `dot_print` | `void dot_print(DotSlice* args)` |
| `dot_println` | `void dot_println(DotSlice* args)` |
| `dot_eprint` | `void dot_eprint(DotSlice* args)` |
| `dot_input` | `DotString* dot_input(DotString* prompt)` |
| `dot_len` | `int64_t dot_len(DotAny collection)` |
| `dot_typeof` | `DotString* dot_typeof(DotAny val)` |
| `dot_assert_fail` | `void dot_assert_fail(DotString* msg)` |
| `dot_panic` | `void dot_panic(DotString* msg)` |
| `dot_pow_int` | `int64_t dot_pow_int(int64_t a, int64_t b)` |
| `dot_pow_float` | `double dot_pow_float(double a, double b)` |

### Dynamic dispatch (dyn.h/dyn.c)
| Function | Signature |
|----------|-----------|
| `dot_dyn_method` | `void* dot_dyn_method(DotDyn* d, int32_t idx)` |

---

## 4. Task List

| # | Task | Target files | Est. lines | Dependencies |
|---|------|--------------|------------|--------------|
| 6.1 | Master header + types | `dot_runtime.h`, `arc.h`, `arc.c` | 190 | — |
| 6.2 | String runtime | `string.h`, `string.c` | 190 | 6.1 |
| 6.3 | Slice runtime | `array.h`, `array.c` | 230 | 6.1 |
| 6.4 | Map runtime | `map.h`, `map.c` | 240 | 6.1 |
| 6.5 | Arena allocator | `arena.h`, `arena.c` | 110 | 6.1 |
| 6.6 | Closure + Iterator | `closure.h`, `closure.c`, `iter.h`, `iter.c` | 190 | 6.1 |
| 6.7 | Channel + Future | `channel.h`, `channel.c`, `future.h`, `future.c` | 130 | 6.1 |
| 6.8 | Weak + Dynamic | `weak.h`, `weak.c`, `dyn.h`, `dyn.c` | 140 | 6.1 |
| 6.9 | Builtins + init | `builtins.h`, `builtins.c`, `dot_runtime.c` | 230 | 6.2–6.8 |
| 6.10 | Integration test: compile + run a .dot file | `testdata/hello.dot` | 50 | all above |

Total: ~1700 lines.

---

## 5. Design Decisions

- **DotAny** is `void*` — values are always pointers (boxed) or cast integers (int stored as intptr_t)
- **All heap types** embed `DotRefcnt` as first member
- **Strings** are immutable, null-terminated, length-prefixed
- **Slices** are growable arrays with cap tracking
- **Maps** use open-addressing hash table (FNV-1a hash)
- **v0.1 stubs**: channel/future use single-threaded polling, no real threading
- **Assert/Panic**: print message + abort()
