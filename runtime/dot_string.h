#ifndef DOT_STRING_H
#define DOT_STRING_H

#include_next <string.h>
#include "arc.h"

typedef struct DotSlice DotSlice;

typedef struct DotString {
    DotRefcnt rc;
    int64_t len;
    char data[];
} DotString;

DotString* dot_string_from_lit(const char* ptr, int64_t len);
DotString* dot_string_from_bytes(DotSlice* bytes);
DotSlice* dot_string_to_bytes(DotString* s);
DotString* dot_string_concat(DotString* a, DotString* b);

#endif
