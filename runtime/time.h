#ifndef DOT_TIME_H
#define DOT_TIME_H

#include_next <time.h>
#include_next <string.h>
#include <stdint.h>
#include "dot_string.h"

typedef struct {
    int64_t seconds;
    int32_t nanos;
} DotTimestamp;

DotTimestamp dot_time_now(void);
void         dot_time_sleep(int64_t milliseconds);
DotString*   dot_time_format(DotTimestamp t, DotString* fmt);
int64_t      dot_time_since_ms(DotTimestamp since);
int64_t      dot_time_until_ms(DotTimestamp until);

#endif

