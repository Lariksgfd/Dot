#ifndef DOT_MATH_H
#define DOT_MATH_H

#include <stdint.h>
#include <math.h>

int64_t dot_math_abs_i64(int64_t x);
double  dot_math_abs_f64(double x);
double  dot_math_sqrt(double x);
double  dot_math_sin(double x);
double  dot_math_cos(double x);
double  dot_math_tan(double x);
double  dot_math_asin(double x);
double  dot_math_acos(double x);
double  dot_math_atan(double x);
double  dot_math_atan2(double y, double x);
int64_t dot_math_min_i64(int64_t a, int64_t b);
double  dot_math_min_f64(double a, double b);
int64_t dot_math_max_i64(int64_t a, int64_t b);
double  dot_math_max_f64(double a, double b);
double  dot_math_floor(double x);
double  dot_math_ceil(double x);
double  dot_math_round(double x);
double  dot_math_log(double x);
double  dot_math_log2(double x);
double  dot_math_log10(double x);
double  dot_math_hypot(double x, double y);
double  dot_math_pow_f64(double a, double b);

#endif
