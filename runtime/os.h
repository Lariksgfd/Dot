#ifndef DOT_OS_H
#define DOT_OS_H

#include <stdint.h>
#include "dot_string.h"
#include "array.h"

DotString* dot_os_getenv(DotString* name);
int64_t    dot_os_setenv(DotString* name, DotString* value);
DotString* dot_os_getcwd(void);
void       dot_os_exit(int64_t code);
DotSlice*  dot_os_args(void);
DotString* dot_os_platform(void);

#endif

