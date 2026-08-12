#include "crypto.h"
#include <windows.h>
#include <wincrypt.h>
#include <stdio.h>
#include <string.h>

static DotString* hash_data(ALG_ID alg, DotString* input) {
    HCRYPTPROV hProv = 0;
    HCRYPTHASH hHash = 0;
    DWORD cbHashSize = 0, dwCount = sizeof(DWORD);
    BYTE *pbHash = NULL;
    DotString* result = NULL;

    if (!CryptAcquireContext(&hProv, NULL, NULL, PROV_RSA_AES, CRYPT_VERIFYCONTEXT)) {
        return dot_string_from_lit("", 0); // Error
    }

    if (!CryptCreateHash(hProv, alg, 0, 0, &hHash)) {
        CryptReleaseContext(hProv, 0);
        return dot_string_from_lit("", 0);
    }

    if (!CryptHashData(hHash, (const BYTE*)input->data, (DWORD)input->len, 0)) {
        CryptDestroyHash(hHash);
        CryptReleaseContext(hProv, 0);
        return dot_string_from_lit("", 0);
    }

    if (!CryptGetHashParam(hHash, HP_HASHSIZE, (BYTE*)&cbHashSize, &dwCount, 0)) {
        CryptDestroyHash(hHash);
        CryptReleaseContext(hProv, 0);
        return dot_string_from_lit("", 0);
    }

    pbHash = (BYTE*)malloc(cbHashSize);
    if (pbHash) {
        if (CryptGetHashParam(hHash, HP_HASHVAL, pbHash, &cbHashSize, 0)) {
            char* hex_str = (char*)malloc(cbHashSize * 2 + 1);
            if (hex_str) {
                for (DWORD i = 0; i < cbHashSize; i++) {
                    sprintf(hex_str + i * 2, "%02x", pbHash[i]);
                }
                hex_str[cbHashSize * 2] = '\0';
                result = dot_string_from_lit(hex_str, cbHashSize * 2);
                free(hex_str);
            }
        }
        free(pbHash);
    }

    if (!result) result = dot_string_from_lit("", 0);

    CryptDestroyHash(hHash);
    CryptReleaseContext(hProv, 0);

    return result;
}

DotString* Dot_sha256(DotString* input) {
    if (!input) return dot_string_from_lit("", 0);
    return hash_data(CALG_SHA_256, input);
}

DotString* Dot_md5(DotString* input) {
    if (!input) return dot_string_from_lit("", 0);
    return hash_data(CALG_MD5, input);
}
