#ifndef __HASH_H__
#define __HASH_H__


typedef unsigned long long (*hash_func_t)(const char *, unsigned long long);

unsigned long long hash_mad_a(const char *, unsigned long long);
unsigned long long hash_mad_b(const char *, unsigned long long);
unsigned long long hash_mix_splitmix_prehash(const char *, unsigned long long);
unsigned long long hash_mix_murmur_prehash(const char *, unsigned long long);
unsigned long long hash_mix_fnv_splitmix(const char *, unsigned long long);
unsigned long long hash_universal_raw(const char *, unsigned long long);
unsigned long long hash_universal_splitmix(const char *, unsigned long long);
unsigned long long hash_universal_fnv_murmur(const char *, unsigned long long);

#endif //__HASH_H__
