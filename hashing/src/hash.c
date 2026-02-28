#include <stdint.h>

#define M_MUL 2654435769U

#define A_MUL1 314159265
#define B_MUL1 271828183

#define A_MUL2 161803399
#define B_MUL2 141421357

#define A_MUL3 577215664
#define B_MUL3 693147181


static unsigned long long prehash_string(const char *str)
{
    unsigned long long h = 0;

    for (int32_t i = 0; str[i] != '\0'; ++i)
        h = h * M_MUL + str[i];

    return h;
}

static unsigned long long mad_hash(
    const char *key,
    const unsigned long long limit,
    const unsigned long long a_mul,
    const unsigned long long b_mul
)
{
    return (a_mul * prehash_string(key) + b_mul) % limit;
}

unsigned long long mad_hash1(const char *key, const unsigned long long limit)
{
    return mad_hash(key, limit, A_MUL1, B_MUL1);
}

unsigned long long mad_hash2(const char *key, const unsigned long long limit)
{
    return mad_hash(key, limit, A_MUL2, B_MUL2);
}

unsigned long long mad_hash3(const char *key, const unsigned long long limit)
{
    return mad_hash(key, limit, A_MUL3, B_MUL3);
}