#include <stdlib.h>

#define M_MUL 2654435769U

#define A_MUL1 314159265
#define B_MUL1 271828183

#define A_MUL2 161803399
#define B_MUL2 141421357

#define FNV_OFFSET 1469598103934665603ULL
#define FNV_PRIME 1099511628211ULL

#define PRIME_P1 2305843009213693951ULL
#define PRIME_P2 9223372036854775783ULL
#define PRIME_P3 18446744073709551557ULL

#define A_U1 11995408973635179863ULL
#define B_U1 10150724397891781847ULL

#define A_U2 6364136223846793005ULL
#define B_U2 1442695040888963407ULL

#define A_U3 3202034522624059733ULL
#define B_U3 3935559000370003845ULL


static unsigned long long prehash_string(const char *str)
{
    unsigned long long h = 0;

    for (size_t i = 0; str[i] != '\0'; ++i)
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

static unsigned long long mix_splitmix64(unsigned long long x)
{
    x += 0x9e3779b97f4a7c15ULL;
    x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9ULL;
    x = (x ^ (x >> 27)) * 0x94d049bb133111ebULL;
    return x ^ (x >> 31);
}

static unsigned long long mix_murmur3_fmix64(unsigned long long x)
{
    x ^= x >> 33;
    x *= 0xff51afd7ed558ccdULL;
    x ^= x >> 33;
    x *= 0xc4ceb9fe1a85ec53ULL;
    x ^= x >> 33;
    return x;
}

static unsigned long long fnv1a_hash(const char *str)
{
    unsigned long long h = FNV_OFFSET;

    for (size_t i = 0; str[i] != '\0'; ++i)
    {
        h ^= (unsigned char)str[i];
        h *= FNV_PRIME;
    }

    return h;
}

static unsigned long long universal_hash(
    const unsigned long long x,
    const unsigned long long limit,
    const unsigned long long a,
    const unsigned long long b,
    const unsigned long long p
)
{
    const unsigned long long mod_p = (unsigned long long)(((__uint128_t)a * x + b) % p);
    return mod_p % limit;
}

unsigned long long hash_mad_a(const char *key, const unsigned long long limit)
{
    return mad_hash(key, limit, A_MUL1, B_MUL1);
}

unsigned long long hash_mad_b(const char *key, const unsigned long long limit)
{
    return mad_hash(key, limit, A_MUL2, B_MUL2);
}

unsigned long long hash_mix_splitmix_prehash(const char *key, const unsigned long long limit)
{
    return mix_splitmix64(prehash_string(key)) % limit;
}

unsigned long long hash_mix_murmur_prehash(const char *key, const unsigned long long limit)
{
    return mix_murmur3_fmix64(prehash_string(key)) % limit;
}

unsigned long long hash_mix_fnv_splitmix(const char *key, const unsigned long long limit)
{
    return mix_splitmix64(fnv1a_hash(key)) % limit;
}

unsigned long long hash_universal_raw(const char *key, const unsigned long long limit)
{
    const unsigned long long x = prehash_string(key);
    return universal_hash(x, limit, A_U1, B_U1, PRIME_P1);
}

unsigned long long hash_universal_splitmix(const char *key, const unsigned long long limit)
{
    const unsigned long long x = mix_splitmix64(prehash_string(key));
    return universal_hash(x, limit, A_U2, B_U2, PRIME_P2);
}

unsigned long long hash_universal_fnv_murmur(const char *key, const unsigned long long limit)
{
    const unsigned long long x = mix_murmur3_fmix64(fnv1a_hash(key));
    return universal_hash(x, limit, A_U3, B_U3, PRIME_P3);
}