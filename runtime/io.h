#ifndef DOT_IO_H
#define DOT_IO_H

#include <stdint.h>
#include <stdio.h>
#include "arc.h"
#include "dot_string.h"

typedef struct DotFile DotFile;
struct DotFile {
    DotRefcnt rc;
    FILE* handle;
    int8_t owned;
};

DotFile*   dot_file_open(DotString* path, DotString* mode);
void       dot_file_close(DotFile* f);
DotString* dot_file_read(DotFile* f, int64_t n);
DotString* dot_file_read_all(DotFile* f);
int64_t    dot_file_write(DotFile* f, DotString* data);
int64_t    dot_file_write_all(DotFile* f, DotString* data);

DotFile* dot_file_stdin(void);
DotFile* dot_file_stdout(void);
DotFile* dot_file_stderr(void);

void dot_print_str(DotString* s);

#endif

