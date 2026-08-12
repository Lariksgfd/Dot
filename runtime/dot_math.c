#include "dot_math.h"
#include <math.h>
#include <stdint.h>
#include <stdlib.h>

int64_t dot_math_abs_i64(int64_t x) { return x < 0 ? -x : x; }
double  dot_math_abs_f64(double x)  { return fabs(x); }
double  dot_math_sqrt(double x)     { return sqrt(x); }
double  dot_math_sin(double x)      { return sin(x); }
double  dot_math_cos(double x)      { return cos(x); }
double  dot_math_tan(double x)      { return tan(x); }
double  dot_math_asin(double x)     { return asin(x); }
double  dot_math_acos(double x)     { return acos(x); }
double  dot_math_atan(double x)     { return atan(x); }
double  dot_math_atan2(double y, double x) { return atan2(y, x); }

int64_t dot_math_min_i64(int64_t a, int64_t b) { return a < b ? a : b; }
double  dot_math_min_f64(double a, double b)   { return a < b ? a : b; }
int64_t dot_math_max_i64(int64_t a, int64_t b) { return a > b ? a : b; }
double  dot_math_max_f64(double a, double b)   { return a > b ? a : b; }

double  dot_math_floor(double x)  { return floor(x); }
double  dot_math_ceil(double x)   { return ceil(x); }
double  dot_math_round(double x)  { return round(x); }
double  dot_math_log(double x)    { return log(x); }
double  dot_math_log2(double x)   { return log2(x); }
double  dot_math_log10(double x)  { return log10(x); }
double  dot_math_hypot(double x, double y) { return hypot(x, y); }
double  dot_math_pow_f64(double a, double b) { return pow(a, b); }
