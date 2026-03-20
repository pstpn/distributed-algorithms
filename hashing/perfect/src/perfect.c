#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "hash.h"
#include "perfect.h"
#include "primes.h"


static const hash_func_t HASH_FUNCS[] = {
    hash_mad_a,
    hash_mad_b,
    hash_mix_splitmix_prehash,
    hash_mix_murmur_prehash,
    hash_mix_fnv_splitmix,
    hash_universal_raw,
    hash_universal_splitmix,
    hash_universal_fnv_murmur
};

static void free_cells(ceil_value_t **, size_t);
static int8_t build_table(perfect_hash_table_t *, const char **, const double *, size_t);

int8_t create_perfect_table(const hash_func_t hash, const size_t size, perfect_hash_table_t **ht)
{
    *ht = malloc(sizeof(perfect_hash_table_t));
    if (!*ht)
        return ERR_ALLOC;

    (*ht)->size = find_next_prime(size);
    (*ht)->not_null_cells = 0;
    (*ht)->hash = hash ? hash : hash_mad_a;
    (*ht)->cells = calloc((*ht)->size, sizeof(ceil_value_t *));
    if (!(*ht)->cells)
    {
        free(*ht);
        return ERR_ALLOC;
    }

    return SUCCESS;
}

static void free_entries(char **keys, double *values, const size_t count)
{
    if (keys)
        for (size_t i = 0; i < count; ++i)
            free(keys[i]);

    free(keys);
    free(values);
}

static int8_t parse_line(char *line, char **out_text, double *out_value)
{
    if (!line || !out_text || !out_value)
        return ERR_PARSE;

    size_t len = strlen(line);
    while (len > 0 && (line[len - 1] == '\n' || line[len - 1] == '\r'))
        line[--len] = '\0';

    if (line[0] == '\0')
        return ERR_PARSE;

    char *comma = strrchr(line, ',');
    if (!comma || comma[1] == '\0')
        return ERR_PARSE;

    *comma = '\0';
    const char *value_str = comma + 1;

    char *end_ptr = NULL;
    const double value = strtod(value_str, &end_ptr);
    if (end_ptr == value_str || *end_ptr != '\0')
        return ERR_PARSE;

    *out_text = line;
    *out_value = value;
    return SUCCESS;
}

int8_t perfect_table_from_csv(const char *csv_path, const hash_func_t hash, perfect_hash_table_t **ht)
{
    if (!csv_path || !ht)
        return ERR_EMPTY_INPUT;

    FILE *file = fopen(csv_path, "r");
    if (!file)
        return ERR_OPEN_FILE;

    size_t cap = 0;
    char *line = NULL;

    if (getline(&line, &cap, file) < 0)
    {
        free(line);
        fclose(file);
        return ERR_PARSE;
    }

    size_t keys_count = 0;
    size_t keys_cap = 1024;
    char **keys = malloc(sizeof(char *) * keys_cap);
    double *values = malloc(sizeof(double) * keys_cap);
    if (!keys || !values)
    {
        free(line);
        fclose(file);
        free(keys);
        free(values);
        return ERR_ALLOC;
    }

    while (getline(&line, &cap, file) >= 0)
    {
        char *text = NULL;
        double value = 0.0;

        const int8_t rc = parse_line(line, &text, &value);
        if (rc != SUCCESS)
        {
            free_entries(keys, values, keys_count);
            free(line);
            fclose(file);
            return rc;
        }

        if (keys_count == keys_cap)
        {
            const size_t new_cap = keys_cap * 2;
            char **new_keys = realloc(keys, sizeof(char *) * new_cap);
            if (!new_keys)
            {
                free_entries(keys, values, keys_count);
                free(line);
                fclose(file);
                return ERR_ALLOC;
            }
            keys = new_keys;

            double *new_values = realloc(values, sizeof(double) * new_cap);
            if (!new_values)
            {
                free_entries(keys, values, keys_count);
                free(line);
                fclose(file);
                return ERR_ALLOC;
            }
            values = new_values;
            keys_cap = new_cap;
        }

        keys[keys_count] = strdup(text);
        if (!keys[keys_count])
        {
            free_entries(keys, values, keys_count);
            free(line);
            fclose(file);
            return ERR_ALLOC;
        }

        values[keys_count] = value;
        ++keys_count;
    }

    int8_t rc = create_perfect_table(hash, keys_count, ht);
    if (rc != SUCCESS)
    {
        free_entries(keys, values, keys_count);
        free(line);
        fclose(file);
        return rc;
    }

    rc = build_table(*ht, (const char **)keys, values, keys_count);
    if (rc != SUCCESS)
    {
        free_perfect_table(*ht);
        *ht = NULL;
    }

    free_entries(keys, values, keys_count);
    free(line);
    fclose(file);

    return rc;
}

static int8_t build_candidate_table(
    const hash_func_t hash,
    const size_t size,
    const char **keys,
    const double *values,
    const size_t count,
    ceil_value_t ***out_cells
)
{
    ceil_value_t **cells = calloc(size, sizeof(ceil_value_t *));
    if (!cells)
        return ERR_ALLOC;

    for (size_t i = 0; i < count; ++i)
    {
        const unsigned long long idx = hash(keys[i], size);
        if (cells[idx])
        {
            free_cells(cells, size);
            return ERR_PARSE;
        }

        ceil_value_t *cell = malloc(sizeof(ceil_value_t));
        if (!cell)
        {
            free_cells(cells, size);
            return ERR_ALLOC;
        }

        cell->is_hash_table = false;
        cell->data.pair.key = strdup(keys[i]);
        if (!cell->data.pair.key)
        {
            free(cell);
            free_cells(cells, size);
            return ERR_ALLOC;
        }
        cell->data.pair.value = values[i];
        cells[idx] = cell;
    }

    *out_cells = cells;
    return SUCCESS;
}

static int8_t build_table(
    perfect_hash_table_t *ht,
    const char **keys,
    const double *values,
    const size_t count
)
{
    if (!ht || (!keys && count > 0) || (!values && count > 0) || ht->not_null_cells != 0)
        return ERR_EMPTY_INPUT;

    if (count == 0)
        return SUCCESS;

    for (size_t candidate_size = ht->size; ; candidate_size = find_next_prime(candidate_size * 2 + 1))
    {
        for (size_t i = 0; i < sizeof(HASH_FUNCS) / sizeof(HASH_FUNCS[0]); ++i)
        {
            ceil_value_t **candidate = NULL;
            const int8_t rc = build_candidate_table(HASH_FUNCS[i], candidate_size, keys, values, count, &candidate);
            if (rc == SUCCESS)
            {
                free(ht->cells);
                ht->cells = candidate;
                ht->size = candidate_size;
                ht->hash = HASH_FUNCS[i];
                ht->not_null_cells = count;
                return SUCCESS;
            }

            if (rc == ERR_ALLOC)
                return ERR_ALLOC;
        }
    }
}

int8_t perfect_get(const perfect_hash_table_t *ht, const char *key, double *value)
{
    if (!ht || !key || !value)
        return ERR_EMPTY_INPUT;

    const unsigned long long h = ht->hash(key, ht->size);
    const ceil_value_t *cell = ht->cells[h];
    if (!cell)
        return ERR_NOT_FOUND;

    if (strcmp(cell->data.pair.key, key) == 0)
    {
        *value = cell->data.pair.value;
        return SUCCESS;
    }

    return ERR_NOT_FOUND;
}

static void free_cells(ceil_value_t **cells, const size_t size)
{
    if (!cells)
        return;

    for (size_t i = 0; i < size; ++i)
    {
        if (!cells[i])
            continue;

        if (cells[i]->is_hash_table)
            free_perfect_table(cells[i]->data.hash_table);
        else
            free(cells[i]->data.pair.key);

        free(cells[i]);
    }

    free(cells);
}

void free_perfect_table(perfect_hash_table_t *ht)
{
    if (!ht)
        return;

    free_cells(ht->cells, ht->size);
    free(ht);
}
