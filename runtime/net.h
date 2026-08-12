#ifndef DOT_NET_H
#define DOT_NET_H

#include "dot_runtime.h"
#include "dot_string.h"
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

int64_t dot_net_init(void);
void dot_net_cleanup(void);

int64_t dot_net_bind(DotString* host, int64_t port);
int64_t dot_net_listen(int64_t fd, int64_t backlog);
int64_t dot_net_accept(int64_t fd);
int64_t dot_net_connect(DotString* host, int64_t port);

DotString* dot_net_read(int64_t fd, int64_t n);
int64_t dot_net_write(int64_t fd, DotString* data);
void dot_net_close(int64_t fd);

#ifdef __cplusplus
}
#endif

#endif // DOT_NET_H
