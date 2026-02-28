#ifndef __HASH_H__
#define __HASH_H__


typedef unsigned long long (*hash_func_t)(const char *, unsigned long long);

unsigned long long mad_hash1(const char *, unsigned long long);
unsigned long long mad_hash2(const char *, unsigned long long);
unsigned long long mad_hash3(const char *, unsigned long long);

#endif //__HASH_H__
