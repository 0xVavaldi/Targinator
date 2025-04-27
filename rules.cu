#include <cuda_runtime.h>
#include <stdio.h>
#include <stdint.h>

#ifdef _WIN32
#define DLL_EXPORT __declspec(dllexport)
#else
#define DLL_EXPORT __attribute__((visibility("default")))
#endif

#define MAX_LEN 32
#define THREADS 512
// nvcc --shared -o rules.dll rules.cu -Xcompiler "/MD" -link -cudart static
// nvcc -shared -o librules.dll rules.cu

// XXHash64 constants
#define PRIME64_1 0x9E3779B185EBCA87ULL
#define PRIME64_2 0xC2B2AE3D27D4EB4FULL
#define PRIME64_3 0x165667B19E3779F9ULL
#define PRIME64_4 0x85EBCA77C2B2AE63ULL
#define PRIME64_5 0x27D4EB2F165667C5ULL
#define CUDA_CHECK(err) do { \
    if (err != cudaSuccess) { \
        fprintf(stderr, "CUDA error: %s at %s:%d\n", \
                cudaGetErrorString(err), __FILE__, __LINE__); \
        exit(1); \
    } \
} while(0)

__device__ bool binarySearchGPU(const uint64_t* array, uint64_t length, uint64_t target) {
    int left = 0;
    int right = length - 1;
    while (left <= right) {
        int mid = left + (right - left) / 2;
        uint64_t midVal = array[mid];
        if (midVal == target) return true;
        if (midVal < target) left = mid + 1;
        else right = mid - 1;
    }
    return false;
}

// 64-bit rotation function
__device__ uint64_t rotl64(uint64_t x, int r) {
    return (x << r) | (x >> (64 - r));
}

// XXHash round operation
__device__ uint64_t round(uint64_t acc, uint64_t input) {
    acc += input * PRIME64_2;
    acc = rotl64(acc, 31);
    acc *= PRIME64_1;
    return acc;
}

// Merge round for combining accumulators
__device__ uint64_t mergeRound(uint64_t acc, uint64_t val) {
    val = round(0, val);
    acc ^= val;
    acc = acc * PRIME64_1 + PRIME64_4;
    return acc;
}

// Read 32-bit value from unaligned memory
__device__ uint32_t read32(const void* ptr) {
    const uint8_t* byte_ptr = (const uint8_t*)ptr;
    return (uint32_t)byte_ptr[0] | ((uint32_t)byte_ptr[1] << 8) |
           ((uint32_t)byte_ptr[2] << 16) | ((uint32_t)byte_ptr[3] << 24);
}

// Read 64-bit value from unaligned memory (little-endian)
__device__ uint64_t read64(const void* ptr) {
    const uint8_t* byte_ptr = (const uint8_t*)ptr;
    return (uint64_t)byte_ptr[0] | ((uint64_t)byte_ptr[1] << 8) |
           ((uint64_t)byte_ptr[2] << 16) | ((uint64_t)byte_ptr[3] << 24) |
           ((uint64_t)byte_ptr[4] << 32) | ((uint64_t)byte_ptr[5] << 40) |
           ((uint64_t)byte_ptr[6] << 48) | ((uint64_t)byte_ptr[7] << 56);
}

// XXHash64 computation for a single string
__device__ uint64_t xxhash64(const char* input, int len, uint64_t seed) {
    const char* p = input;
    const char* const end = p + len;
    uint64_t h64;

    if (len >= 32) {
        const char* const limit = end - 32;
        uint64_t v1 = seed + PRIME64_1 + PRIME64_2;
        uint64_t v2 = seed + PRIME64_2;
        uint64_t v3 = seed + 0;
        uint64_t v4 = seed - PRIME64_1;

        // Process 32-byte chunks
        do {
            v1 = round(v1, read64(p)); p += 8;
            v2 = round(v2, read64(p)); p += 8;
            v3 = round(v3, read64(p)); p += 8;
            v4 = round(v4, read64(p)); p += 8;
        } while (p <= limit);

        // Combine accumulators
        h64 = rotl64(v1, 1) + rotl64(v2, 7) + rotl64(v3, 12) + rotl64(v4, 18);
        h64 = mergeRound(h64, v1);
        h64 = mergeRound(h64, v2);
        h64 = mergeRound(h64, v3);
        h64 = mergeRound(h64, v4);
    } else {
        h64 = seed + PRIME64_5;
    }

    h64 += (uint64_t)len;

    // Process remaining bytes
    while (p + 8 <= end) {
        uint64_t k1 = read64(p);
        k1 *= PRIME64_2;
        k1 = rotl64(k1, 31);
        k1 *= PRIME64_1;
        h64 ^= k1;
        h64 = rotl64(h64, 27) * PRIME64_1 + PRIME64_4;
        p += 8;
    }

    // Process last 4 bytes
    if (p + 4 <= end) {
        h64 ^= (uint64_t)read32(p) * PRIME64_1;
        h64 = rotl64(h64, 23) * PRIME64_2 + PRIME64_3;
        p += 4;
    }

    // Process remaining bytes
    while (p < end) {
        h64 ^= (*p) * PRIME64_5;
        h64 = rotl64(h64, 11) * PRIME64_1;
        p++;
    }

    // Final avalanche
    h64 ^= h64 >> 33;
    h64 *= PRIME64_2;
    h64 ^= h64 >> 29;
    h64 *= PRIME64_3;
    h64 ^= h64 >> 32;

    return h64;
}

// CUDA kernel to compute hashes for all strings
extern "C" {
// 'l' Rule: converts all (ASCII) characters to lowercase.
__global__ void lowerCaseKernel(char *words, int *lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char *word = &words[idx * MAX_LEN];
        for (int i = 0; i < len; i++) {
            // Simple ASCII conversion: if character is A-Z, add 32.
            if (word[i] >= 'A' && word[i] <= 'Z') {
                word[i] = word[i] + 32;
            }
        }
    }
}

// 'u' Rule: converts all (ASCII) characters to uppercase.
__global__ void upperCaseKernel(char *words, int *lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char *word = &words[idx * MAX_LEN];
        for (int i = 0; i < len; i++) {
            // Simple ASCII conversion: if character is a-z, subtract 32.
            if (word[i] >= 'a' && word[i] <= 'z') {
                word[i] = word[i] - 32;
            }
        }
    }
}

// 'c' Rule: Capitalize first letter, lowercase rest
__global__ void capitalizeKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len == 0) return;

        // Lowercase all characters
        for (int i = 0; i < len; ++i) {
            if (word[i] >= 'A' && word[i] <= 'Z') {
                word[i] += 32;
            }
        }
        // Uppercase first character
        if (word[0] >= 'a' && word[0] <= 'z') {
            word[0] -= 32;
        }
    }
}

// 'C' Rule: Lowercase first letter, uppercase rest
__global__ void invertCapitalizeKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len == 0) return;

        // Uppercase all characters
        for (int i = 0; i < len; ++i) {
            if (word[i] >= 'a' && word[i] <= 'z') {
                word[i] -= 32;
            }
        }
        // Lowercase first character
        if (word[0] >= 'A' && word[0] <= 'Z') {
            word[0] += 32;
        }
    }
}

// 't' Rule: Toggle case of all characters
__global__ void toggleCaseKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        for (int i = 0; i < len; i++) {
            if (word[i] >= 'A' && word[i] <= 'Z') {
                word[i] += 32;
            } else if (word[i] >= 'a' && word[i] <= 'z') {
                word[i] -= 32;
            }
        }
    }
}

// 'q' Rule: Duplicate each character
__global__ void duplicateCharsKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (2 * len > MAX_LEN - 1) return;

        // Duplicate characters
        for (int i = len - 1; i >= 0; --i) {
            word[2 * i] = word[i];
            word[2 * i + 1] = word[i];
        }
        lengths[idx] = 2 * len;
        word[2 * len] = '\0';
    }
}

// 'r' Rule: Reverse Word
__global__ void reverseKernel(char *words, int *lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char *word = &words[idx * MAX_LEN];
        // Swap characters from beginning and end
        for (int i = 0; i < len / 2; i++) {
            char temp = word[i];
            word[i] = word[len - 1 - i];
            word[len - 1 - i] = temp;
        }
    }
}

// 'k' Rule: Swap first two characters
__global__ void swapFirstTwoKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len >= 2) {
            char temp = word[0];
            word[0] = word[1];
            word[1] = temp;
        }
    }
}

// 'K' Rule: Swap last two characters
__global__ void swapLastTwoKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len >= 2) {
            char temp = word[len-1];
            word[len-1] = word[len-2];
            word[len-2] = temp;
        }
    }
}

// 'd' Rule: Duplicate word
__global__ void duplicateWordKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len * 2 < MAX_LEN) {
            for (int i = 0; i < len; i++) {
                word[len + i] = word[i];
            }
            lengths[idx] = len * 2;
            word[len * 2] = '\0';
        }
    }
}

// 'f' Rule: Append reversed string
__global__ void reflectKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (2 * len > MAX_LEN - 1) return;

        // Append reversed characters
        for (int i = 0; i < len; ++i) {
            word[len + i] = word[len - 1 - i];
        }
        lengths[idx] = 2 * len;
        word[2 * len] = '\0';
    }
}

// '{' Rule: Rotate left (move first char to end)
__global__ void rotateLeftKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len <= 1) return;

        char first = word[0];
        for (int i = 0; i < len - 1; ++i) {
            word[i] = word[i + 1];
        }
        word[len - 1] = first;
    }
}

// '}' Rule: Rotate right (move last char to start)
__global__ void rotateRightKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len <= 1) return;

        char last = word[len - 1];
        for (int i = len - 1; i > 0; --i) {
            word[i] = word[i - 1];
        }
        word[0] = last;
    }
}

// '[' Rule: Delete first character
__global__ void deleteFirstKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len <= 0) return;
        for (int i = 0; i < len - 1; i++) {
            word[i] = word[i + 1];
        }
        lengths[idx] = len - 1;
        word[len - 1] = '\0';
    }
}

// ']' Rule: Delete last character
__global__ void deleteLastKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len <= 0) return;

        lengths[idx] = len - 1;
        word[len - 1] = '\0';
    }
}

// 'E' Rule: Title case (capitalize after spaces)
__global__ void titleCaseKernel(char* words, int* lengths, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];

        // Lowercase all
        for (int i = 0; i < len; ++i) {
            if (word[i] >= 'A' && word[i] <= 'Z') word[i] += 32;
        }

        // Capitalize first and after spaces
        if (len > 0 && word[0] >= 'a' && word[0] <= 'z') {
            word[0] -= 32;
        }
        for (int i = 1; i < len; ++i) {
            if (word[i - 1] == ' ' && word[i] >= 'a' && word[i] <= 'z') {
                word[i] -= 32;
            }
        }
    }
}

// 'T' Rule: Toggle case at position
__global__ void togglePositionKernel(char* words, int* lengths, int pos, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (pos < 0 || pos >= len) return;

        if (word[pos] >= 'a' && word[pos] <= 'z') {
            word[pos] -= 32;
        } else if (word[pos] >= 'A' && word[pos] <= 'Z') {
            word[pos] += 32;
        }
    }
}

// 'p' Rule: Repeat word N times
__global__ void repeatWordKernel(char* words, int* lengths, int count, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        int newLen = len * (count + 1);
        if (newLen >= MAX_LEN) newLen = MAX_LEN - 1;

        for (int i = 1; i <= count && (i * len) < newLen; i++) {
            for (int j = 0; j < len && (i * len + j) < newLen; j++) {
                word[i * len + j] = word[j];
            }
        }
        lengths[idx] = newLen;
        word[newLen] = '\0';
    }
}

// 'D' Rule: Delete character at position
__global__ void deletePositionKernel(char* words, int* lengths, int pos, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (pos < 0 || pos >= len) return;

        for (int i = pos; i < len - 1; ++i) {
            word[i] = word[i + 1];
        }
        lengths[idx] = len - 1;
        word[len - 1] = '\0';
    }
}

// 'z' Rule: Prepend first character multiple times
__global__ void prependFirstCharKernel(char* words, int* lengths, int count, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len == 0 || count <= 0) return;

        int newLen = len + count;
        if (newLen > MAX_LEN - 1) newLen = MAX_LEN - 1;
        char first = word[0];

        // Shift existing characters
        for (int i = newLen - 1; i >= count; --i) {
            word[i] = word[i - count];
        }
        // Prepend first character
        for (int i = 0; i < count; ++i) {
            word[i] = first;
        }
        lengths[idx] = newLen;
        word[newLen] = '\0';
    }
}

// 'Z' Rule: Append last character N times
__global__ void appendLastCharKernel(char* words, int* lengths, int count, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len == 0) return;

        char last = word[len - 1];
        int newLen = len + count;
        if (newLen >= MAX_LEN) newLen = MAX_LEN - 1;

        for (int i = len; i < newLen; i++) {
            word[i] = last;
        }
        lengths[idx] = newLen;
        word[newLen] = '\0';
    }
}

// ''' Rule: Truncate at position
__global__ void truncateAtKernel(char* words, int* lengths, int pos, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        if (pos < len) {
            lengths[idx] = pos;
            words[idx * MAX_LEN + pos] = '\0';
        }
    }
}

// 's' Rule: replaces all occurrences of oldChar with newChar.
__global__ void substitutionKernel(char *words, int *lengths, char oldChar, char newChar, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char *word = &words[idx * MAX_LEN];
        for (int i = 0; i < len; i++) {
            if (word[i] == oldChar) {
                word[i] = newChar;
            }
        }
    }
}

// 'S' Rule: replaces first occurrence of oldChar with newChar.
__global__ void substitutionFirstKernel(char *words, int *lengths, char oldChar, char newChar, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char *word = &words[idx * MAX_LEN];
        for (int i = 0; i < len; i++) {
            if (word[i] == oldChar) {
                word[i] = newChar;
                break;
            }
        }
    }
}

// '$' Rule: Appends a given character to the end of each word.
__global__ void appendKernel(char *words, int *lengths, char appendChar, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        if (len < MAX_LEN - 1) {
            words[idx * MAX_LEN + len] = appendChar;
            words[idx * MAX_LEN + len + 1] = '\0';
            lengths[idx] = len + 1;
        }
    }
}

// '^' Rule: Prepends a given character to the start of each word.
__global__ void prependKernel(char *words, int *lengths, char prefixChar, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        if (len < MAX_LEN - 1) {
            char *word = &words[idx * MAX_LEN];
            // Shift the word one character to the right (including the terminator)
            for (int i = len; i >= 0; i--) {
                word[i + 1] = word[i];
            }
            word[0] = prefixChar;
            lengths[idx] = len + 1;
        }
    }
}

// 'y' Rule: Prepend substring
__global__ void prependSubstrKernel(char* words, int* lengths, int count, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        int prependLen = min(count, len);
        int newLen = len + prependLen;

        if (newLen >= MAX_LEN) {
            newLen = MAX_LEN - 1;
            prependLen = newLen - len;
        }

        // Shift right
        for (int i = newLen - 1; i >= prependLen; i--) {
            word[i] = word[i - prependLen];
        }
        // Copy prefix
        for (int i = 0; i < prependLen; i++) {
            word[i] = word[i + prependLen];
        }
        lengths[idx] = newLen;
        word[newLen] = '\0';
    }
}

// 'Y' Rule: Append substring
__global__ void appendSubstrKernel(char* words, int* lengths, int count, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        int appendLen = min(count, len);
        int newLen = len + appendLen;

        if (newLen >= MAX_LEN) {
            newLen = MAX_LEN - 1;
            appendLen = newLen - len;
        }

        for (int i = 0; i < appendLen; i++) {
            word[len + i] = word[len - appendLen + i];
        }
        lengths[idx] = newLen;
        word[newLen] = '\0';
    }
}

// 'L' Rule: Bitwise shift left at position
__global__ void bitShiftLeftKernel(char* words, int* lengths, int pos, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (pos >= 0 && pos < len) {
            word[pos] = word[pos] << 1;
        }
    }
}

// 'R' Rule: Bitwise shift right at position
__global__ void bitShiftRightKernel(char* words, int* lengths, int pos, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (pos >= 0 && pos < len) {
            word[pos] = word[pos] >> 1;
        }
    }
}

// '-' Rule: Decrement character at position
__global__ void decrementCharKernel(char* words, int* lengths, int pos, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (pos >= 0 && pos < len) {
            word[pos]--;
        }
    }
}

// '+' Rule: Increment character at position
__global__ void incrementCharKernel(char* words, int* lengths, int pos, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (pos >= 0 && pos < len) {
            word[pos]++;
        }
    }
}

// '@' Rule: Delete all instances of character
__global__ void deleteAllCharKernel(char* words, int* lengths, char target, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        int newIndex = 0;
        for (int i = 0; i < len; i++) {
            if (word[i] != target) {
                word[newIndex++] = word[i];
            }
        }
        lengths[idx] = newIndex;
        word[newIndex] = '\0';
    }
}

// '.' Rule: Swap with next character
__global__ void swapNextKernel(char* words, int* lengths, int pos, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (pos >= 0 && pos < len - 1) {
            char temp = word[pos];
            word[pos] = word[pos + 1];
            word[pos + 1] = temp;
        }
    }
}

// ',' Rule: Swap with next character
__global__ void swapLastKernel(char* words, int* lengths, int pos, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (pos >= 0 && pos < len - 1) {
            char temp = word[pos];
            word[pos] = word[pos - 1];
            word[pos - 1] = temp;
        }
    }
}

// 'e' Rule: Title case with separator
__global__ void titleSeparatorKernel(char* words, int* lengths, char separator, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len == 0) return;
        // Convert all characters to lowercase
        for (int i = 0; i < len; i++) {
            // Simple ASCII conversion: if character is A-Z, add 32.
            if (word[i] >= 'A' && word[i] <= 'Z') {
                word[i] = word[i] + 32;
            }
        }
        // Capitalize first character
        if (word[0] >= 'a' && word[0] <= 'z') {
            word[0] = word[0] - 32;
        }
        // Process trigger characters
        // -1 to prevent overflow in i+1 check and 1 to not double capitalize and gain small % performance
        // I can imagine this being undesired - feel free to report
        for (int i = 1; i < len - 1; ++i) {
            if (word[i] == separator) {
                word[i+1] = word[i+1] - 32;
            }
        }
    }
}

// 'i' Rule: Insert string at position
__global__ void insertKernel(char* words, int* lengths, int pos, char insert_char, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len == 0 || pos > len) return;
        if (len + 1 >= MAX_LEN) return;

        // Shift characters right
        for (int i = len; i > pos; --i) {
            word[i] = word[i - 1];
        }
        word[pos] = insert_char;

        lengths[idx] = len + 1;
        word[len + 1] = '\0';
    }
}

// 'O' Rule: Omit Range M starting at pos N (ONM)
__global__ void OmitKernel(char* words, int* lengths, int pos, int count, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len <= 0 || pos >= len) return;

        int delete_count = min(count, len - pos);
        for (int i = pos; i < len - delete_count; ++i) {
            word[i] = word[i + delete_count];
        }
        lengths[idx] = len - delete_count;
        word[len - delete_count] = '\0';
    }
}

// 'o' Rule: Overwrite at position
__global__ void overwriteKernel(char* words, int* lengths, int pos, char replace_char, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];

        // Check for empty string or invalid position
        if (len <= 0 || pos < 0 || pos >= len) return;

        // Overwrite single character
        word[pos] = replace_char;
    }
}

// '*' Rule: Swap any two
__global__ void swapAnyKernel(char* words, int* lengths, int pos, int replace_pos, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len == 0 || pos < 0 || replace_pos < 0 || pos >= len || replace_pos >= len) return;

        // Swap
        char temp = word[pos];
        word[pos] = word[replace_pos];
        word[replace_pos] = temp;
    }
}


// 'x' Rule: Delete all except a slice i.e. extract a substring
__global__ void extractKernel(char* words, int* lengths, int pos, int count, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        if (len <= 0 || pos >= len) return;
        int keep_count = min(count, len - pos);
        // First, shift the kept slice to the beginning
        for (int i = 0; i < keep_count; ++i) {
            word[i] = word[pos + i];
        }
        // Truncate after the kept portion
        lengths[idx] = keep_count;
        word[keep_count] = '\0';
    }
}

// '<' Rule: Reject words longer than or equal to count
__global__ void rejectLessKernel(char* words, int* lengths, int count, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        if (lengths[idx] >= count) {
            lengths[idx] = 0;
            words[idx * MAX_LEN] = '\0';
        }
    }
}

// '>' Rule: Reject words shorter than or equal to count
__global__ void rejectGreaterKernel(char* words, int* lengths, int count, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        if (lengths[idx] <= count) {
            lengths[idx] = 0;
            words[idx * MAX_LEN] = '\0';
        }
    }
}

// '_' Rule: Reject words with exact length
__global__ void rejectEqualKernel(char* words, int* lengths, int count, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        if (lengths[idx] == count) {
            lengths[idx] = 0;
            words[idx * MAX_LEN] = '\0';
        }
    }
}

// '!' Rule: Reject words containing specific character
__global__ void rejectContainKernel(char* words, int* lengths, char contain_char, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];

        for (int i = 0; i < len; ++i) {
            if (word[i] == contain_char) {
                lengths[idx] = 0;
                word[0] = '\0';
                return;
            }
        }
    }
}

// '/' Rule: Reject words not containing specific character
__global__ void rejectNotContainKernel(char* words, int* lengths, char contain_char, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        bool found = false;

        for (int i = 0; i < len; ++i) {
            if (word[i] == contain_char) {
                found = true;
                break;
            }
        }

        if (!found) {
            lengths[idx] = 0;
            word[0] = '\0';
        }
    }
}

// '3' Rule: Toggle case after nth instance of character
__global__ void toggleWithNSeparatorKernel(char* words, int* lengths, char separator_char, int separator_num, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx < numWords) {
        int len = lengths[idx];
        char* word = &words[idx * MAX_LEN];
        int count = 0;
        int toggle_pos = -1;

        // Find nth instance
        for (int i = 0; i < len; ++i) {
            if (word[i] == separator_char) {
                if (count == separator_num) {
                    toggle_pos = i + 1;
                    break;
                }
                count++;
            }
        }

        // Toggle next character if found and within bounds
        if (toggle_pos > 0 && toggle_pos < len) {
            if (word[toggle_pos] >= 'a' && word[toggle_pos] <= 'z') {
                word[toggle_pos] = word[toggle_pos] - 32;
            } else if (word[toggle_pos] >= 'A' && word[toggle_pos] <= 'Z') {
                word[toggle_pos] = word[toggle_pos] + 32;
            }
        }
    }
}



//---------------------------------------------------------------------
// Host Wrappers to Launch Kernels
//---------------------------------------------------------------------
// l
DLL_EXPORT
void applyLowerCase(char *d_words, int *d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    lowerCaseKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// u
DLL_EXPORT
void applyUpperCase(char *d_words, int *d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    upperCaseKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// c
DLL_EXPORT
void applyCapitalize(char* d_words, int* d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    capitalizeKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// C
DLL_EXPORT
void applyInvertCapitalize(char* d_words, int* d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    invertCapitalizeKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// t
DLL_EXPORT
void applyToggleCase(char* d_words, int* d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    toggleCaseKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// q
DLL_EXPORT
void applyDuplicateChars(char* d_words, int* d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    duplicateCharsKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// r
DLL_EXPORT
void applyReverse(char *d_words, int *d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    reverseKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// k
DLL_EXPORT
void applySwapFirstTwo(char *d_words, int *d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    swapFirstTwoKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// K
DLL_EXPORT
void applySwapLastTwo(char *d_words, int *d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    swapLastTwoKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// d
DLL_EXPORT
void applyDuplicate(char *d_words, int *d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    duplicateWordKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// f
DLL_EXPORT
void applyReflect(char* d_words, int* d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    reflectKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// {
DLL_EXPORT
void applyRotateLeft(char* d_words, int* d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    rotateLeftKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
 // }
DLL_EXPORT
void applyRotateRight(char* d_words, int* d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    rotateRightKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// [
DLL_EXPORT
void applyDeleteFirst(char* d_words, int* d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    deleteFirstKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// ]
DLL_EXPORT
void applyDeleteLast(char* d_words, int* d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    deleteLastKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// E
DLL_EXPORT
void applyTitleCase(char* d_words, int* d_lengths, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    titleCaseKernel<<<blocks, THREADS>>>(d_words, d_lengths, numWords);
    cudaDeviceSynchronize();
}
// T
DLL_EXPORT
void applyTogglePosition(char* d_words, int* d_lengths, int pos, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    togglePositionKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, numWords);
    cudaDeviceSynchronize();
}
// p
DLL_EXPORT
void applyRepeatWord(char* d_words, int* d_lengths, int count, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    repeatWordKernel<<<blocks, THREADS>>>(d_words, d_lengths, count, numWords);
    cudaDeviceSynchronize();
}
// D
DLL_EXPORT
void applyDeletePosition(char* d_words, int* d_lengths, int pos, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    deletePositionKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, numWords);
    cudaDeviceSynchronize();
}
// z
DLL_EXPORT
void applyPrependFirstChar(char* d_words, int* d_lengths, int count, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    prependFirstCharKernel<<<blocks, THREADS>>>(d_words, d_lengths, count, numWords);
    cudaDeviceSynchronize();
}
// Z
DLL_EXPORT
void applyAppendLastChar(char* d_words, int* d_lengths, int count, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    prependFirstCharKernel<<<blocks, THREADS>>>(d_words, d_lengths, count, numWords);
    cudaDeviceSynchronize();
}
// '
DLL_EXPORT
void applyTruncateAt(char* d_words, int* d_lengths, int pos, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    truncateAtKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, numWords);
    cudaDeviceSynchronize();
}
// s
DLL_EXPORT
void applySubstitution(char *d_words, int *d_lengths, char oldChar, char newChar, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    substitutionKernel<<<blocks, THREADS>>>(d_words, d_lengths, oldChar, newChar, numWords);
    cudaDeviceSynchronize();
}
// S
DLL_EXPORT
void applySubstitutionFirst(char *d_words, int *d_lengths, char oldChar, char newChar, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    substitutionFirstKernel<<<blocks, THREADS>>>(d_words, d_lengths, oldChar, newChar, numWords);
    cudaDeviceSynchronize();
}
// $
DLL_EXPORT
void applyAppend(char *d_words, int *d_lengths, char appendChar, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    appendKernel<<<blocks, THREADS>>>(d_words, d_lengths, appendChar, numWords);
    cudaDeviceSynchronize();
}
// ^
DLL_EXPORT
void applyPrepend(char *d_words, int *d_lengths, char prefixChar, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    prependKernel<<<blocks, THREADS>>>(d_words, d_lengths, prefixChar, numWords);
    cudaDeviceSynchronize();
}
// y
DLL_EXPORT
void applyPrependPrefixSubstr(char *d_words, int *d_lengths, int count, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    prependSubstrKernel<<<blocks, THREADS>>>(d_words, d_lengths, count, numWords);
    cudaDeviceSynchronize();
}
// Y
DLL_EXPORT
void applyAppendSuffixSubstr(char *d_words, int *d_lengths, int count, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    appendSubstrKernel<<<blocks, THREADS>>>(d_words, d_lengths, count, numWords);
    cudaDeviceSynchronize();
}
// L
DLL_EXPORT
void applyBitShiftLeft(char *d_words, int *d_lengths, int pos, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    bitShiftLeftKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, numWords);
    cudaDeviceSynchronize();
}
// R
DLL_EXPORT
void applyBitShiftRight(char *d_words, int *d_lengths, int pos, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    bitShiftRightKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, numWords);
    cudaDeviceSynchronize();
}
// -
DLL_EXPORT
void applyDecrementChar(char *d_words, int *d_lengths, int pos, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    decrementCharKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, numWords);
    cudaDeviceSynchronize();
}
// +
DLL_EXPORT
void applyIncrementChar(char *d_words, int *d_lengths, int pos, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    incrementCharKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, numWords);
    cudaDeviceSynchronize();
}
// @
DLL_EXPORT
void applyDeleteAllChar(char *d_words, int *d_lengths, char deleteChar, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    deleteAllCharKernel<<<blocks, THREADS>>>(d_words, d_lengths, deleteChar, numWords);
    cudaDeviceSynchronize();
}
// .
DLL_EXPORT
void applySwapNext(char *d_words, int *d_lengths, int pos, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    swapNextKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, numWords);
    cudaDeviceSynchronize();
}
// ,
DLL_EXPORT
void applySwapLast(char *d_words, int *d_lengths, int pos, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    swapLastKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, numWords);
    cudaDeviceSynchronize();
}
// e
DLL_EXPORT
void applyTitleSeparator(char *d_words, int *d_lengths, char separator, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    titleSeparatorKernel<<<blocks, THREADS>>>(d_words, d_lengths, separator, numWords);
    cudaDeviceSynchronize();
}
// i
DLL_EXPORT
void applyInsert(char* d_words, int* d_lengths, int pos, char insert_char, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    insertKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, insert_char, numWords);
    cudaDeviceSynchronize();
}
// O
DLL_EXPORT
void applyOmit(char* d_words, int* d_lengths, int pos, int count, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    OmitKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, count, numWords);
    cudaDeviceSynchronize();
}
// o
DLL_EXPORT
void applyOverwrite(char* d_words, int* d_lengths, int pos, char replace_char, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    overwriteKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, replace_char, numWords);
    cudaDeviceSynchronize();
}
// *
DLL_EXPORT
void applySwapAny(char* d_words, int* d_lengths, int pos, int replace_pos, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    swapAnyKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, replace_pos, numWords);
    cudaDeviceSynchronize();
}
// x
DLL_EXPORT
void applyExtract(char* d_words, int* d_lengths, int pos, int count, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    extractKernel<<<blocks, THREADS>>>(d_words, d_lengths, pos, count, numWords);
    cudaDeviceSynchronize();
}
// <
DLL_EXPORT
void applyRejectLess(char* d_words, int* d_lengths, int count, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    rejectLessKernel<<<blocks, THREADS>>>(d_words, d_lengths, count, numWords);
    cudaDeviceSynchronize();
}

// >
DLL_EXPORT
void applyRejectGreater(char* d_words, int* d_lengths, int count, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    rejectGreaterKernel<<<blocks, THREADS>>>(d_words, d_lengths, count, numWords);
    cudaDeviceSynchronize();
}

// _
DLL_EXPORT
void applyRejectEqual(char* d_words, int* d_lengths, int count, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    rejectEqualKernel<<<blocks, THREADS>>>(d_words, d_lengths, count, numWords);
    cudaDeviceSynchronize();
}

// !
DLL_EXPORT
void applyRejectContain(char* d_words, int* d_lengths, char contain_char, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    rejectContainKernel<<<blocks, THREADS>>>(d_words, d_lengths, contain_char, numWords);
    cudaDeviceSynchronize();
}

// /
DLL_EXPORT
void applyRejectNotContain(char* d_words, int* d_lengths, char contain_char, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    rejectNotContainKernel<<<blocks, THREADS>>>(d_words, d_lengths, contain_char, numWords);
    cudaDeviceSynchronize();
}

// 3
DLL_EXPORT
void applyToggleWithNSeparator(char* d_words, int* d_lengths, char separator_char, int separator_num, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    toggleWithNSeparatorKernel<<<blocks, THREADS>>>(d_words, d_lengths, separator_char, separator_num, numWords);
    cudaDeviceSynchronize();
}

// Existing helper functions
DLL_EXPORT
void allocateOriginalDictMemoryOnGPU(
    char **d_originalDict, int **d_originalDictLengths,
    char *h_originalDict, int *h_originalDictLengths, int numWords
) {
    CUDA_CHECK(cudaMalloc((void**)d_originalDict, numWords * MAX_LEN * sizeof(char)));
    CUDA_CHECK(cudaMalloc((void**)d_originalDictLengths, numWords * sizeof(int)));

    CUDA_CHECK(cudaMemcpy(*d_originalDict, h_originalDict, numWords * MAX_LEN * sizeof(char), cudaMemcpyHostToDevice));
    CUDA_CHECK(cudaMemcpy(*d_originalDictLengths, h_originalDictLengths, numWords * sizeof(int), cudaMemcpyHostToDevice));
    cudaDeviceSynchronize();
}

DLL_EXPORT
void allocateProcessedDictMemoryOnGPU(
    char **d_processedDict, int **d_processedDictLengths, int numWords
) {
    CUDA_CHECK(cudaMalloc((void**)d_processedDict, numWords * MAX_LEN * sizeof(char)));
    CUDA_CHECK(cudaMalloc((void**)d_processedDictLengths, numWords * sizeof(int)));
    cudaDeviceSynchronize();
}

DLL_EXPORT
void allocateHashesMemoryOnGPU(
    uint64_t **d_originalHashes, int originalCount,
    uint64_t **d_compareHashes, int compareCount,
    uint64_t *h_originalHashes, uint64_t *h_compareHashes
) {
    CUDA_CHECK(cudaMalloc((void**)d_originalHashes, originalCount * sizeof(uint64_t)));
    CUDA_CHECK(cudaMalloc((void**)d_compareHashes, compareCount * sizeof(uint64_t)));

    CUDA_CHECK(cudaMemcpy(*d_originalHashes, h_originalHashes, originalCount * sizeof(uint64_t), cudaMemcpyHostToDevice));
    CUDA_CHECK(cudaMemcpy(*d_compareHashes, h_compareHashes, compareCount * sizeof(uint64_t), cudaMemcpyHostToDevice));
    cudaDeviceSynchronize();
}

DLL_EXPORT
void ResetProcessedDictMemoryOnGPU(
    char **d_originalDict, int **d_originalDictLengths,
    char **d_processedDict, int **d_processedDictLengths,
    int numWords
) {
    CUDA_CHECK(cudaMemcpy(*d_processedDict, *d_originalDict, numWords * MAX_LEN * sizeof(char), cudaMemcpyDeviceToDevice));
    CUDA_CHECK(cudaMemcpy(*d_processedDictLengths, *d_originalDictLengths, numWords * sizeof(int), cudaMemcpyDeviceToDevice));
    cudaDeviceSynchronize();
}

DLL_EXPORT
void copyMemoryBackToHost(
    char *h_processedDict, int *h_processedDictLengths,
    char **d_processedDict, int **d_processedDictLengths, int numWords) {
    CUDA_CHECK(cudaMemcpy(h_processedDict, *d_processedDict, numWords * MAX_LEN * sizeof(char), cudaMemcpyDeviceToHost));
    CUDA_CHECK(cudaMemcpy(h_processedDictLengths, *d_processedDictLengths, numWords * sizeof(int), cudaMemcpyDeviceToHost));
    cudaDeviceSynchronize();
}

DLL_EXPORT
void freeOriginalMemoryOnGPU(int *d_originalDict, int *d_originalDictLengths) {
    CUDA_CHECK(cudaFree(d_originalDict));
    CUDA_CHECK(cudaFree(d_originalDictLengths));
    cudaDeviceSynchronize();
}

DLL_EXPORT
void freeProcessedMemoryOnGPU(char *d_processedDict, int *d_processedDictLengths, uint64_t *d_hitCount) {
    CUDA_CHECK(cudaFree(d_processedDictLengths));
    CUDA_CHECK(cudaFree(d_processedDict));
    CUDA_CHECK(cudaFree(d_hitCount));
    cudaDeviceSynchronize();
}

__global__ void xxhashKernel(char* d_processedDict, int* d_processedDictLengths , uint64_t seed, uint64_t* hashes, int numWords) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx >= numWords) return;

    char* input = &d_processedDict[idx * MAX_LEN];
    hashes[idx] = xxhash64(input, d_processedDictLengths[idx], seed);
}

__global__ void xxhashWithCheckKernel(
    char* d_processedDict,
    int* d_processedLengths,
    uint64_t seed,
    const uint64_t* d_originalHashes,
    uint64_t originalCount,
    const uint64_t* d_compareHashes,
    uint64_t compareCount,
    uint64_t* hitCount
) {
    const int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx >= originalCount) return;

    // Compute hash
    char* word = &d_processedDict[idx * MAX_LEN];
    const uint64_t hash = xxhash64(word, d_processedLengths[idx], seed);
    // Atomic increment if valid hit
    const bool inCompare = binarySearchGPU(d_compareHashes, compareCount, hash);
    if (inCompare && !binarySearchGPU(d_originalHashes, originalCount, hash)) {
        atomicAdd(reinterpret_cast<unsigned long long*>(hitCount), 1ULL);
    }
}

// Exported function to launch the kernel
DLL_EXPORT
void computeXXHashes(char *d_processedDict, int *d_processedDictLengths, uint64_t seed, uint64_t* d_hashes, int numWords) {
    int blocks = (numWords + THREADS - 1) / THREADS;
    xxhashKernel<<<blocks, THREADS>>>(d_processedDict, d_processedDictLengths , seed, d_hashes, numWords);
    cudaDeviceSynchronize();
}

DLL_EXPORT
void computeXXHashesWithCheck(
    char *d_processedDict, int *d_processedDictLengths, uint64_t seed,
    const uint64_t* d_originalHashes, int originalHashCount,
    const uint64_t* d_compareHashes, int compareHashCount,
    uint64_t* d_hitCount
) {
    int blocks = (originalHashCount + THREADS - 1) / THREADS;
    cudaMemset(d_hitCount, 0, sizeof(uint64_t));
    cudaDeviceSynchronize();
    xxhashWithCheckKernel<<<blocks, THREADS>>>(
        d_processedDict, d_processedDictLengths, seed,
        d_originalHashes, originalHashCount,
        d_compareHashes, compareHashCount,
        d_hitCount
    );
}

} // extern "C"