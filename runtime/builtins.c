#include "dot_runtime.h"
#include <stdio.h>
#include <math.h>

static bool is_ptr(DotAny val) {
    return ((uintptr_t)val) > 0xFFFF;
}

static bool is_slice(DotAny val) {
    if (!is_ptr(val)) return false;
    DotRefcnt* rc = (DotRefcnt*)val;
    return rc->count > 0 && rc->count < 1000000
        ;
}

static bool is_string(DotAny val) {
    if (!is_ptr(val)) return false;
    DotRefcnt* rc = (DotRefcnt*)val;
    DotString* s = (DotString*)val;
    return rc->count > 0 && rc->count < 1000000
        && s->len >= 0 && s->len < 100000000
        ;
}

static void print_val(FILE* f, DotAny val) {
    if (is_string(val)) {
        DotString* s = (DotString*)val;
        fwrite(s->data, 1, (size_t)s->len, f);
    } else {
        fprintf(f, "%ld", (long)(intptr_t)val);
    }
}

void dot_print(DotSlice* args) {
    if (!args) return;
    int64_t n = dot_slice_len(args);
    for (int64_t i = 0; i < n; i++) {
        print_val(stdout, dot_slice_get(args, i));
    }
}

void dot_println(DotSlice* args) {
    dot_print(args);
    printf("\n");
}

void dot_eprint(DotSlice* args) {
    if (!args) return;
    int64_t n = dot_slice_len(args);
    for (int64_t i = 0; i < n; i++) {
        print_val(stderr, dot_slice_get(args, i));
    }
}

DotString* dot_input(DotString* prompt) {
    if (prompt) {
        fwrite(prompt->data, 1, (size_t)prompt->len, stdout);
    }
    char buf[4096];
    if (!fgets(buf, sizeof(buf), stdin)) {
        buf[0] = '\0';
    }
    size_t len = strlen(buf);
    if (len > 0 && buf[len - 1] == '\n') {
        buf[--len] = '\0';
    }
    return dot_string_from_lit(buf, (int64_t)len);
}

int64_t dot_len(DotAny collection) {
    if (!collection) return 0;
    if (!is_ptr(collection)) return -1;
    if (is_slice(collection)) {
        return dot_slice_len((DotSlice*)collection);
    }
    if (is_string(collection)) {
        return ((DotString*)collection)->len;
    }
    return -1;
}

DotString* dot_typeof(DotAny val) {
    (void)val;
    return dot_string_from_lit("unknown", 7);
}

void dot_assert_fail(DotString* msg) {
    fprintf(stderr, "assertion failed: %s\n", msg ? msg->data : "");
    abort();
}

void dot_panic(DotString* msg) {
    fprintf(stderr, "panic: %s\n", msg ? msg->data : "");
    abort();
}

int64_t dot_pow_int(int64_t a, int64_t b) {
    if (b < 0) return 0;
    int64_t result = 1;
    for (int64_t i = 0; i < b; i++) {
        result *= a;
    }
    return result;
}

double dot_pow_float(double a, double b) {
    return pow(a, b);
}
