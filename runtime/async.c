#include "async.h"
#include <stdlib.h>
#include <stdio.h>

#ifdef _WIN32
#include <windows.h>
#define MAX_TASKS 128

static void* main_fiber;
static void* fiber_tasks[MAX_TASKS];
static int task_count = 0;
static int current_task = -1;
static int active_tasks = 0;

static void CALLBACK fiber_run(void* param) {
    void (*fn)(void) = (void (*)(void))param;
    fn();
    fiber_tasks[current_task] = NULL; // Mark as done
    active_tasks--;
    SwitchToFiber(main_fiber);
}

void async_init(void) {
    main_fiber = ConvertThreadToFiber(NULL);
    task_count = 0;
    active_tasks = 0;
}

void async_spawn(void (*fn)(void)) {
    if (task_count < MAX_TASKS) {
        fiber_tasks[task_count] = CreateFiber(0, fiber_run, (void*)fn);
        task_count++;
        active_tasks++;
    }
}

void async_yield(void) {
    if (current_task >= 0) {
        SwitchToFiber(main_fiber);
    }
}

void async_run(void) {
    while (active_tasks > 0) {
        for (int i = 0; i < task_count; i++) {
            if (fiber_tasks[i] != NULL) {
                current_task = i;
                SwitchToFiber(fiber_tasks[i]);
            }
        }
    }
    current_task = -1;
}
#else
// POSIX ucontext version or dummy
void async_init(void) {}
void async_spawn(void (*fn)(void)) { fn(); }
void async_yield(void) {}
void async_run(void) {}
#endif
