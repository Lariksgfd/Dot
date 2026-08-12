#include "os.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <windows.h>
#include <shellapi.h>
#else
#include <unistd.h>
#endif

extern DotAny dot_alloc(int32_t size);

DotString* dot_os_getenv(DotString* name) {
    if (!name || name->len == 0) return dot_string_from_lit("", 0);
#ifdef _WIN32
    char buf[4096];
    DWORD len = GetEnvironmentVariableA(name->data, buf, (DWORD)sizeof(buf));
    if (len == 0 || len >= sizeof(buf)) return dot_string_from_lit("", 0);
    return dot_string_from_lit(buf, (int64_t)len);
#else
    const char* val = getenv(name->data);
    if (!val) return dot_string_from_lit("", 0);
    return dot_string_from_lit(val, (int64_t)strlen(val));
#endif
}

int64_t dot_os_setenv(DotString* name, DotString* value) {
    if (!name || !value) return -1;
#ifdef _WIN32
    BOOL ok = SetEnvironmentVariableA(name->data, value->data);
    return ok ? 0 : -1;
#else
    int r = setenv(name->data, value->data, 1);
    return (int64_t)r;
#endif
}

DotString* dot_os_getcwd(void) {
    char buf[4096];
#ifdef _WIN32
    DWORD len = GetCurrentDirectoryA((DWORD)sizeof(buf), buf);
    if (len == 0 || len >= sizeof(buf)) return dot_string_from_lit(".", 1);
    return dot_string_from_lit(buf, (int64_t)len);
#else
    if (!getcwd(buf, sizeof(buf))) return dot_string_from_lit(".", 1);
    return dot_string_from_lit(buf, (int64_t)strlen(buf));
#endif
}

void dot_os_exit(int64_t code) {
    exit((int)code);
}

DotSlice* dot_os_args(void) {
    DotSlice* s = dot_slice_from_array(0, NULL);
#ifdef _WIN32
    {
        int argc = 0;
        LPWSTR* wargv = CommandLineToArgvW(GetCommandLineW(), &argc);
        if (wargv) {
            for (int i = 0; i < argc; i++) {
                int len = WideCharToMultiByte(CP_UTF8, 0, wargv[i], -1, NULL, 0, NULL, NULL);
                if (len > 0) {
                    char* buf = (char*)malloc((size_t)len);
                    if (buf) {
                        WideCharToMultiByte(CP_UTF8, 0, wargv[i], -1, buf, len, NULL, NULL);
                        DotString* arg = dot_string_from_lit(buf, (int64_t)(len - 1));
                        free(buf);
                        dot_slice_push(s, (DotAny)arg);
                    }
                }
            }
            LocalFree(wargv);
        }
    }
#else
    {
        /* fallback: use __argv if available (MinGW cross-compile) */
        extern char** __argv;
        extern int __argc;
        for (int i = 0; i < __argc; i++) {
            DotString* arg = dot_string_from_lit(__argv[i], (int64_t)strlen(__argv[i]));
            dot_slice_push(s, (DotAny)arg);
        }
    }
#endif
    return s;
}

DotString* dot_os_platform(void) {
#ifdef _WIN32
    return dot_string_from_lit("windows", 7);
#elif defined(__APPLE__)
    return dot_string_from_lit("macos", 5);
#elif defined(__linux__)
    return dot_string_from_lit("linux", 5);
#else
    return dot_string_from_lit("unknown", 7);
#endif
}
