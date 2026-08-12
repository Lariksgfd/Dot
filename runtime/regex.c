#include "regex.h"
#include <stdlib.h>
#include <string.h>

extern DotAny dot_alloc(int32_t size);

DotRegexObj* dot_regex_compile(DotString* pattern) {
    if (!pattern || pattern->len == 0) return NULL;
    char* r = (char*)malloc(pattern->len + 1);
    if (!r) return NULL;
    memcpy(r, pattern->data, pattern->len);
    r[pattern->len] = '\0';
    
    DotRegexObj* obj = (DotRegexObj*)dot_alloc((int32_t)sizeof(DotRegexObj));
    obj->handle = r;
    return obj;
}

bool dot_regex_match(DotRegexObj* regex, DotString* text) {
    if (!regex || !regex->handle || !text || text->len == 0) return false;
    char* r = (char*)regex->handle;
    return strstr(text->data, r) != NULL;
}

void dot_regex_free(DotRegexObj* regex) {
    if (regex && regex->handle) {
        free(regex->handle);
        regex->handle = NULL;
    }
    if (regex) {
        dot_release(&regex->rc);
    }
}
