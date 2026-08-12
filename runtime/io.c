#include "io.h"
#include <stdio.h>
#include <stdlib.h>

extern DotAny dot_alloc(int32_t size);

DotFile* dot_file_open(DotString* path, DotString* mode) {
    if (!path || !mode) return NULL;
    FILE* h = fopen(path->data, mode->data);
    if (!h) return NULL;
    DotFile* f = (DotFile*)dot_alloc((int32_t)sizeof(DotFile));
    f->handle = h;
    f->owned = 1;
    return f;
}

void dot_file_close(DotFile* f) {
    if (!f) return;
    if (f->owned && f->handle) {
        fclose(f->handle);
    }
    dot_release(&f->rc);
}

static DotFile* make_std(FILE* h) {
    DotFile* f = (DotFile*)dot_alloc((int32_t)sizeof(DotFile));
    f->handle = h;
    f->owned = 0;
    f->rc.count = INT32_MAX;
    return f;
}

DotFile* dot_file_stdin(void) {
    static DotFile* f = NULL;
    if (!f) f = make_std(stdin);
    return f;
}

DotFile* dot_file_stdout(void) {
    static DotFile* f = NULL;
    if (!f) f = make_std(stdout);
    return f;
}

DotFile* dot_file_stderr(void) {
    static DotFile* f = NULL;
    if (!f) f = make_std(stderr);
    return f;
}

DotString* dot_file_read(DotFile* f, int64_t n) {
    if (!f || !f->handle || n <= 0) return dot_string_from_lit("", 0);
    DotString* s = (DotString*)dot_alloc((int32_t)(sizeof(DotString) + (size_t)n + 1));
    size_t r = fread(s->data, 1, (size_t)n, f->handle);
    s->len = (int64_t)r;
    s->data[r] = '\0';
    return s;
}

DotString* dot_file_read_all(DotFile* f) {
    if (!f || !f->handle) return dot_string_from_lit("", 0);
    long pos = ftell(f->handle);
    if (fseek(f->handle, 0, SEEK_END) != 0) return dot_string_from_lit("", 0);
    long size = ftell(f->handle);
    fseek(f->handle, pos, SEEK_SET);
    if (size <= 0) return dot_string_from_lit("", 0);
    DotString* s = (DotString*)dot_alloc((int32_t)(sizeof(DotString) + (size_t)size + 1));
    size_t r = fread(s->data, 1, (size_t)size, f->handle);
    s->len = (int64_t)r;
    s->data[r] = '\0';
    return s;
}

int64_t dot_file_write(DotFile* f, DotString* data) {
    if (!f || !f->handle || !data || data->len == 0) return 0;
    size_t w = fwrite(data->data, 1, (size_t)data->len, f->handle);
    return (int64_t)w;
}

int64_t dot_file_write_all(DotFile* f, DotString* data) {
    if (!f || !f->handle || !data || data->len == 0) return 0;
    size_t total = 0;
    size_t remaining = (size_t)data->len;
    const char* ptr = data->data;
    while (remaining > 0) {
        size_t w = fwrite(ptr, 1, remaining, f->handle);
        if (w == 0) break;
        total += w;
        ptr += w;
        remaining -= w;
    }
    return (int64_t)total;
}

void dot_print_str(DotString* s) {
    if (s) {
        fwrite(s->data, 1, (size_t)s->len, stdout);
    }
}
