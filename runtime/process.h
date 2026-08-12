#ifndef DOT_PROCESS_H
#define DOT_PROCESS_H

#include <stddef.h>

typedef struct {
    int exit_code;
    char* stdout_str;
} DotProcessResult;

DotProcessResult dot_process_run(const char* cmd, const char** args, int args_count);

#endif // DOT_PROCESS_H
