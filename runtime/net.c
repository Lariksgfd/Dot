#include "net.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

extern void* dot_alloc(int32_t size);

#ifdef _WIN32
    #include <winsock2.h>
    #include <ws2tcpip.h>
    #pragma comment(lib, "ws2_32.lib")
    typedef int socklen_t;
#else
    #include <sys/socket.h>
    #include <netinet/in.h>
    #include <arpa/inet.h>
    #include <unistd.h>
    #include <errno.h>
    #define INVALID_SOCKET -1
    #define SOCKET_ERROR -1
    #define closesocket close
#endif

int64_t dot_net_init(void) {
#ifdef _WIN32
    WSADATA wsaData;
    if (WSAStartup(MAKEWORD(2, 2), &wsaData) != 0) {
        return -1;
    }
#endif
    return 0;
}

void dot_net_cleanup(void) {
#ifdef _WIN32
    WSACleanup();
#endif
}

int64_t dot_net_bind(DotString* host, int64_t port) {
    if (!host || !host->data) return -1;
    
    int sock = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
    if (sock == INVALID_SOCKET) return -1;
    
    int opt = 1;
    setsockopt(sock, SOL_SOCKET, SO_REUSEADDR, (const char*)&opt, sizeof(opt));
    
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons((uint16_t)port);
    
    if (inet_pton(AF_INET, host->data, &addr.sin_addr) <= 0) {
        closesocket(sock);
        return -1;
    }
    
    if (bind(sock, (struct sockaddr*)&addr, sizeof(addr)) == SOCKET_ERROR) {
        closesocket(sock);
        return -1;
    }
    
    return (int64_t)sock;
}

int64_t dot_net_listen(int64_t fd, int64_t backlog) {
    if (listen((int)fd, (int)backlog) == SOCKET_ERROR) {
        return -1;
    }
    return 0;
}

int64_t dot_net_accept(int64_t fd) {
    struct sockaddr_in client_addr;
    socklen_t client_len = sizeof(client_addr);
    int client_sock = accept((int)fd, (struct sockaddr*)&client_addr, &client_len);
    if (client_sock == INVALID_SOCKET) {
        return -1;
    }
    return (int64_t)client_sock;
}

int64_t dot_net_connect(DotString* host, int64_t port) {
    if (!host || !host->data) return -1;
    
    int sock = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
    if (sock == INVALID_SOCKET) return -1;
    
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons((uint16_t)port);
    
    if (inet_pton(AF_INET, host->data, &addr.sin_addr) <= 0) {
        closesocket(sock);
        return -1;
    }
    
    if (connect(sock, (struct sockaddr*)&addr, sizeof(addr)) == SOCKET_ERROR) {
        closesocket(sock);
        return -1;
    }
    
    return (int64_t)sock;
}

DotString* dot_net_read(int64_t fd, int64_t n) {
    if (n <= 0) return dot_string_from_lit("", 0);
    
    DotString* s = (DotString*)dot_alloc((int32_t)(sizeof(DotString) + (size_t)n + 1));
    if (!s) return dot_string_from_lit("", 0);
    
    int r = recv((int)fd, s->data, (int)n, 0);
    if (r <= 0) {
        s->len = 0;
        s->data[0] = '\0';
    } else {
        s->len = (int64_t)r;
        s->data[r] = '\0';
    }
    return s;
}

int64_t dot_net_write(int64_t fd, DotString* data) {
    if (!data || data->len <= 0) return 0;
    
    int w = send((int)fd, data->data, (int)data->len, 0);
    if (w == SOCKET_ERROR) {
        return -1;
    }
    return (int64_t)w;
}

void dot_net_close(int64_t fd) {
    if (fd != -1) {
        closesocket((int)fd);
    }
}
