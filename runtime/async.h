#ifndef DOT_ASYNC_H
#define DOT_ASYNC_H

void async_init(void);
void async_run(void);
void async_spawn(void (*fn)(void));
void async_yield(void);

#endif
