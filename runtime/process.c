#include "process.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <unistd.h>
#include <sys/wait.h>
#endif

DotProcessResult dot_process_run(const char* cmd, const char** args, int args_count) {
    DotProcessResult result;
    result.exit_code = -1;
    result.stdout_str = NULL;

    // Buffer for command line construction
    char cmdline[4096] = {0};
    strcat(cmdline, cmd);
    for (int i = 0; i < args_count; i++) {
        strcat(cmdline, " ");
        strcat(cmdline, args[i]);
    }

#ifdef _WIN32
    HANDLE hReadPipe, hWritePipe;
    SECURITY_ATTRIBUTES sa;
    sa.nLength = sizeof(SECURITY_ATTRIBUTES);
    sa.bInheritHandle = TRUE;
    sa.lpSecurityDescriptor = NULL;

    if (!CreatePipe(&hReadPipe, &hWritePipe, &sa, 0)) {
        return result;
    }
    SetHandleInformation(hReadPipe, HANDLE_FLAG_INHERIT, 0);

    STARTUPINFO si;
    PROCESS_INFORMATION pi;
    ZeroMemory(&si, sizeof(STARTUPINFO));
    si.cb = sizeof(STARTUPINFO);
    si.hStdOutput = hWritePipe;
    si.hStdError = hWritePipe;
    si.dwFlags |= STARTF_USESTDHANDLES;
    ZeroMemory(&pi, sizeof(PROCESS_INFORMATION));

    if (CreateProcessA(NULL, cmdline, NULL, NULL, TRUE, 0, NULL, NULL, &si, &pi)) {
        CloseHandle(hWritePipe);
        
        char buffer[4096];
        DWORD bytesRead;
        size_t total_read = 0;
        result.stdout_str = malloc(1);
        result.stdout_str[0] = '\0';
        
        while (ReadFile(hReadPipe, buffer, sizeof(buffer) - 1, &bytesRead, NULL) && bytesRead != 0) {
            buffer[bytesRead] = '\0';
            result.stdout_str = realloc(result.stdout_str, total_read + bytesRead + 1);
            strcpy(result.stdout_str + total_read, buffer);
            total_read += bytesRead;
        }
        
        WaitForSingleObject(pi.hProcess, INFINITE);
        DWORD exitCode;
        if (GetExitCodeProcess(pi.hProcess, &exitCode)) {
            result.exit_code = exitCode;
        }
        CloseHandle(pi.hProcess);
        CloseHandle(pi.hThread);
        CloseHandle(hReadPipe);
    } else {
        CloseHandle(hReadPipe);
        CloseHandle(hWritePipe);
    }
#else
    int pipefd[2];
    if (pipe(pipefd) == -1) return result;

    pid_t pid = fork();
    if (pid == -1) {
        return result;
    } else if (pid == 0) {
        close(pipefd[0]);
        dup2(pipefd[1], STDOUT_FILENO);
        dup2(pipefd[1], STDERR_FILENO);
        close(pipefd[1]);
        
        const char** exec_args = malloc(sizeof(char*) * (args_count + 2));
        exec_args[0] = cmd;
        for(int i=0; i<args_count; i++) exec_args[i+1] = args[i];
        exec_args[args_count+1] = NULL;
        
        execvp(cmd, (char *const *)exec_args);
        exit(1);
    } else {
        close(pipefd[1]);
        char buffer[4096];
        ssize_t bytesRead;
        size_t total_read = 0;
        result.stdout_str = malloc(1);
        result.stdout_str[0] = '\0';
        
        while ((bytesRead = read(pipefd[0], buffer, sizeof(buffer) - 1)) > 0) {
            buffer[bytesRead] = '\0';
            result.stdout_str = realloc(result.stdout_str, total_read + bytesRead + 1);
            strcpy(result.stdout_str + total_read, buffer);
            total_read += bytesRead;
        }
        close(pipefd[0]);
        int status;
        waitpid(pid, &status, 0);
        if (WIFEXITED(status)) {
            result.exit_code = WEXITSTATUS(status);
        }
    }
#endif

    return result;
}
