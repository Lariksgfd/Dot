#include <time.h>
#include "time.h"
#include <stdio.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <unistd.h>
#include <sys/time.h>
#endif

DotTimestamp dot_time_now(void) {
    DotTimestamp ts = {0, 0};
#ifdef _WIN32
    FILETIME ft;
    ULARGE_INTEGER uli;
    GetSystemTimeAsFileTime(&ft);
    uli.LowPart = ft.dwLowDateTime;
    uli.HighPart = ft.dwHighDateTime;
    uint64_t t = uli.QuadPart;
    ts.seconds = (int64_t)(t / 10000000ULL - 11644473600ULL);
    ts.nanos = (int32_t)((t % 10000000ULL) * 100);
#else
    struct timespec spec;
    clock_gettime(CLOCK_REALTIME, &spec);
    ts.seconds = spec.tv_sec;
    ts.nanos = spec.tv_nsec;
#endif
    return ts;
}

void dot_time_sleep(int64_t milliseconds) {
    if (milliseconds <= 0) return;
#ifdef _WIN32
    Sleep((DWORD)milliseconds);
#else
    usleep((useconds_t)(milliseconds * 1000));
#endif
}

DotString* dot_time_format(DotTimestamp t, DotString* fmt) {
    if (!fmt) return dot_string_from_lit("", 0);
    time_t sec = (time_t)t.seconds;
    struct tm tm_val;
    memset(&tm_val, 0, sizeof(tm_val));
#ifdef _WIN32
    {
        struct tm* tmp = localtime(&sec);
        if (tmp) tm_val = *tmp;
    }
#else
    localtime_r(&sec, &tm_val);
#endif
    char buf[256];
    size_t n = strftime(buf, sizeof(buf), fmt->data, &tm_val);
    return dot_string_from_lit(buf, (int64_t)n);
}

int64_t dot_time_since_ms(DotTimestamp since) {
    DotTimestamp now = dot_time_now();
    int64_t diff_sec = now.seconds - since.seconds;
    int64_t diff_ns = (int64_t)now.nanos - (int64_t)since.nanos;
    return diff_sec * 1000 + diff_ns / 1000000;
}

int64_t dot_time_until_ms(DotTimestamp until) {
    DotTimestamp now = dot_time_now();
    int64_t diff_sec = until.seconds - now.seconds;
    int64_t diff_ns = (int64_t)until.nanos - (int64_t)now.nanos;
    return diff_sec * 1000 + diff_ns / 1000000;
}
