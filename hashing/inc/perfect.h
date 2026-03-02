#ifndef __PERFECT_H__
#define __PERFECT_H__

#include <stdbool.h>
#include <stdlib.h>

#include "hash.h"


enum RC
{
    SUCCESS,
    ERR_ALLOC,
    ERR_EMPTY_INPUT,
    ERR_OPEN_FILE,
    ERR_PARSE,
    ERR_NOT_FOUND
};

struct perfect_hash_table;

typedef struct ceil_value
{
    bool is_hash_table;
    union
    {
        struct
        {
            char *key;
            double value;
        } pair;
        struct perfect_hash_table *hash_table;
    } data;
} ceil_value_t;

typedef struct perfect_hash_table
{
    size_t size;
    size_t not_null_cells;
    hash_func_t hash;
    ceil_value_t **cells;
} perfect_hash_table_t;

int8_t create_perfect_table(hash_func_t, size_t, perfect_hash_table_t **);
int8_t perfect_table_from_csv(const char *, hash_func_t, perfect_hash_table_t **);
int8_t perfect_set(perfect_hash_table_t *, const char *, double);
int8_t perfect_get(const perfect_hash_table_t *, const char *, double *);
void free_perfect_table(perfect_hash_table_t *);

#endif //__PERFECT_H__
