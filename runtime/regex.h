#ifndef DOT_REGEX_H
#define DOT_REGEX_H

#include "dot_runtime.h"

typedef struct DotRegexObj DotRegexObj;
struct DotRegexObj {
    DotRefcnt rc;
    void* handle;
};

DotRegexObj* dot_regex_compile(DotString* pattern);
bool dot_regex_match(DotRegexObj* regex, DotString* text);
void dot_regex_free(DotRegexObj* regex);

#endif // DOT_REGEX_H
