#ifndef DOT_BUILTINS_H
#define DOT_BUILTINS_H

#include "arc.h"

struct DotSlice;
struct DotString;

void dot_print(struct DotSlice* args);
void dot_println(struct DotSlice* args);
void dot_eprint(struct DotSlice* args);
struct DotString* dot_input(struct DotString* prompt);
int64_t dot_len(DotAny collection);
struct DotString* dot_typeof(DotAny val);
void dot_assert_fail(struct DotString* msg);
void dot_panic(struct DotString* msg);
int64_t dot_pow_int(int64_t a, int64_t b);
double dot_pow_float(double a, double b);

#endif
