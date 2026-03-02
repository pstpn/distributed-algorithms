#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#include "perfect.h"
#include "primes.h"
#include "hash.h"


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

static int8_t parse_line(char *line, char **out_text, double *out_value)
{
    if (!line)
        return ERR_PARSE;

    size_t len = strlen(line);
    while (len > 0 && (line[len - 1] == '\n' || line[len - 1] == '\r'))
        line[--len] = '\0';
    if (line[0] == '\0')
        return ERR_PARSE;

    char *comma = strrchr(line, ',');
    if (!comma || comma == line || comma[1] == '\0')
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
    while (getline(&line, &cap, file) >= 0)
    {
        char *text;
        double value;

        const int8_t rc = parse_line(line, &text, &value);
        if (rc != SUCCESS)
        {
            free(line);
            fclose(file);
            return rc;
        }

        ++keys_count;
    }
    const int8_t rc = create_perfect_table(hash, keys_count, ht);
    if (rc != SUCCESS)
    {
        free(line);
        fclose(file);
        return rc;
    }

    fseek(file, 0, SEEK_SET);
    if (getline(&line, &cap, file) < 0)
    {
        free(line);
        fclose(file);
        return ERR_PARSE;
    }

    while (getline(&line, &cap, file) >= 0)
    {
        char *text;
        double value;

        int8_t rc = parse_line(line, &text, &value);
        if (rc != SUCCESS)
        {
            free(line);
            fclose(file);
            return rc;
        }

        rc = perfect_set(*ht, text, value);
        if (rc != SUCCESS)
        {
            free(line);
            fclose(file);
            return rc;
        }
    }

    free(line);
    fclose(file);

    return SUCCESS;
}

static void collect_secondary_table(const perfect_hash_table_t *secondary, char ***keys, double **values)
{
    for (size_t i = 0, pos = 0; i < secondary->size; ++i)
    {
        const ceil_value_t *cell = secondary->cells[i];
        if (!cell)
            continue;

        (*keys)[pos] = cell->data.pair.key;
        (*values)[pos] = cell->data.pair.value;
        ++pos;
    }
}

static int8_t build_candidate_secondary(
    const hash_func_t hash,
    const size_t size,
    char **keys,
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

static int8_t insert_secondary_table(perfect_hash_table_t *secondary, const char *key, const double value)
{
    const unsigned long long new_hash = secondary->hash(key, secondary->size);
    const ceil_value_t *ceil_to_insert = secondary->cells[new_hash];

    if (!ceil_to_insert)
    {
        ceil_value_t *ceil = malloc(sizeof(ceil_value_t));
        if (!ceil)
            return ERR_ALLOC;

        ceil->is_hash_table = false;
        ceil->data.pair.key = strdup(key);
        if (!ceil->data.pair.key)
        {
            free(ceil);
            return ERR_ALLOC;
        }
        ceil->data.pair.value = value;

        secondary->cells[new_hash] = ceil;
        ++secondary->not_null_cells;
        return SUCCESS;
    }
    if (strcmp(ceil_to_insert->data.pair.key, key) == 0)
    {
        secondary->cells[new_hash]->data.pair.value = value;
        return SUCCESS;
    }

    const size_t new_count = secondary->not_null_cells + 1;
    char **keys = malloc(sizeof(char *) * new_count);
    double *values = malloc(sizeof(double) * new_count);
    if (!keys || !values)
    {
        free(keys);
        free(values);
        return ERR_ALLOC;
    }

    collect_secondary_table(secondary, &keys, &values);
    keys[new_count - 1] = (char *)key;
    values[new_count - 1] = value;

    for (size_t candidate_size = secondary->size; ; candidate_size = find_next_prime(candidate_size * 2 + 1))
    {
        for (size_t i = 0; i < sizeof(HASH_FUNCS) / sizeof(HASH_FUNCS[0]); ++i)
        {
            ceil_value_t **candidate = NULL;
            const int8_t rc = build_candidate_secondary(HASH_FUNCS[i], candidate_size,
                                           keys, values, new_count,
                                           &candidate);
            if (rc == SUCCESS)
            {
                free_cells(secondary->cells, secondary->size);
                secondary->cells = candidate;
                secondary->hash = HASH_FUNCS[i];
                secondary->size = candidate_size;
                secondary->not_null_cells = new_count;

                free(keys);
                free(values);
                return SUCCESS;
            }

            if (rc == ERR_ALLOC)
            {
                free(keys);
                free(values);
                return ERR_ALLOC;
            }
        }
    }
}

int8_t perfect_set(perfect_hash_table_t *ht, const char *key, const double value)
{
    const unsigned long long h = ht->hash(key, ht->size);
    ceil_value_t *current_ceil = ht->cells[h];

    if (!current_ceil)
    {
        ceil_value_t *ceil = malloc(sizeof(ceil_value_t));
        if (!ceil)
            return ERR_ALLOC;

        ceil->is_hash_table = false;
        ceil->data.pair.key = strdup(key);
        if (!ceil->data.pair.key)
        {
            free(ceil);
            return ERR_ALLOC;
        }
        ceil->data.pair.value = value;

        ht->cells[h] = ceil;
        ++ht->not_null_cells;
    }
    else if (!current_ceil->is_hash_table)
    {
        if (strcmp(current_ceil->data.pair.key, key) == 0)
        {
            current_ceil->data.pair.value = value;
            return SUCCESS;
        }

        perfect_hash_table_t *child_table;
        int8_t rc = create_perfect_table(hash_mad_a, 2, &child_table);
        if (rc != SUCCESS)
            return rc;

        rc = insert_secondary_table(child_table, current_ceil->data.pair.key, current_ceil->data.pair.value);
        if (rc != SUCCESS)
        {
            free_perfect_table(child_table);
            return rc;
        }

        rc = insert_secondary_table(child_table, key, value);
        if (rc != SUCCESS)
        {
            free_perfect_table(child_table);
            return rc;
        }

        free(current_ceil->data.pair.key);
        current_ceil->is_hash_table = true;
        current_ceil->data.hash_table = child_table;
    }
    else
        return insert_secondary_table(current_ceil->data.hash_table, key, value);

    return SUCCESS;
}

int8_t perfect_get(const perfect_hash_table_t *ht, const char *key, double *value)
{
    if (!value)
        return ERR_EMPTY_INPUT;

    const unsigned long long h = ht->hash(key, ht->size);
    const ceil_value_t *ceil = ht->cells[h];

    if (!ceil)
        return ERR_NOT_FOUND;

    if (ceil->is_hash_table)
        return perfect_get(ceil->data.hash_table, key, value);
    if (strcmp(ceil->data.pair.key, key) == 0)
    {
        *value = ceil->data.pair.value;
        return SUCCESS;
    }

    return ERR_NOT_FOUND;
}

static void free_cells(ceil_value_t **cells, const size_t size)
{
    if (!cells)
        return;

    for (size_t i = 0; i < size; ++i)
        if (cells[i])
        {
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
