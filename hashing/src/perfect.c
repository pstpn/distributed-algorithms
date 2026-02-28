#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#include "perfect.h"
#include "primes.h"
#include "hash.h"


static const hash_func_t HASH_FUNCS[] = {mad_hash1, mad_hash2, mad_hash3};

static perfect_hash_table_t *create_child_table(const char *k1, const char *k2, const size_t size)
{
    for (size_t i = 0; i < sizeof(HASH_FUNCS) / sizeof(HASH_FUNCS[0]); ++i)
    {
        perfect_hash_table_t *child = create_perfect_table(HASH_FUNCS[i], size);
        if (child == NULL)
            return NULL;

        if (child->hash(k1, child->size) != child->hash(k2, child->size))
            return child;

        free_perfect_table(child);
    }

    return NULL;
}

perfect_hash_table_t *create_perfect_table(const hash_func_t hash, const size_t keys_count)
{
    perfect_hash_table_t *hash_table = malloc(sizeof(perfect_hash_table_t));
    if (hash_table == NULL)
        return NULL;

    hash_table->size = find_next_prime(keys_count);
    hash_table->hash = hash != NULL ? hash : mad_hash1;
    hash_table->cells = calloc(hash_table->size, sizeof(ceil_value_t *));
    if (hash_table->cells == NULL)
    {
        free(hash_table);
        return NULL;
    }

    return hash_table;
}

static int8_t parse_line(char *line, char **out_text, double *out_value)
{
    if (line == NULL)
        return ERR_PARSE;

    size_t len = strlen(line);
    while (len > 0 && (line[len - 1] == '\n' || line[len - 1] == '\r'))
        line[--len] = '\0';
    if (line[0] == '\0')
        return ERR_PARSE;

    char *comma = strrchr(line, ',');
    if (comma == NULL || comma == line || comma[1] == '\0')
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
    if (csv_path == NULL || ht == NULL)
        return ERR_EMPTY_INPUT;

    FILE *file = fopen(csv_path, "r");
    if (file == NULL)
        return ERR_OPEN_FILE;

    size_t count = 0;
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

        if (parse_line(line, &text, &value) == SUCCESS)
            ++keys_count;
    }
    *ht = create_perfect_table(hash, keys_count);
    if (*ht == NULL)
        return ERR_ALLOC;

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

        ++count;
    }

    free(line);
    fclose(file);

    return SUCCESS;
}

int8_t perfect_set(const perfect_hash_table_t *ht, const char *key, const double value)
{
    const unsigned long long h = ht->hash(key, ht->size);
    ceil_value_t *current_ceil = ht->cells[h];

    if (current_ceil == NULL)
    {
        ceil_value_t *ceil = malloc(sizeof(ceil_value_t));
        if (ceil == NULL)
            return ERR_ALLOC;

        ceil->is_hash_table = false;
        ceil->data.pair.key = strdup(key);
        if (ceil->data.pair.key == NULL)
        {
            free(ceil);
            return ERR_ALLOC;
        }
        ceil->data.pair.value = value;

        ht->cells[h] = ceil;
    }
    else if (!current_ceil->is_hash_table)
    {
        if (strcmp(current_ceil->data.pair.key, key) == 0)
        {
            current_ceil->data.pair.value = value;
            return SUCCESS;
        }

        perfect_hash_table_t *child_table = create_child_table(current_ceil->data.pair.key, key, ht->size);
        if (child_table == NULL)
        {
            child_table = create_perfect_table(ht->hash, ht->size);
            if (child_table == NULL)
                return ERR_ALLOC;
        }

        int8_t rc = perfect_set(child_table, current_ceil->data.pair.key, current_ceil->data.pair.value);
        if (rc != SUCCESS)
        {
            free_perfect_table(child_table);
            return rc;
        }

        rc = perfect_set(child_table, key, value);
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
        return perfect_set(current_ceil->data.hash_table, key, value);

    return SUCCESS;
}

int8_t perfect_get(const perfect_hash_table_t *ht, const char *key, double *value)
{
    if (!value)
        return ERR_EMPTY_INPUT;

    const unsigned long long h = ht->hash(key, ht->size);
    const ceil_value_t *ceil = ht->cells[h];

    if (ceil == NULL)
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

void free_perfect_table(perfect_hash_table_t *ht)
{
    if (ht == NULL)
        return;

    for (int32_t i = 0; i < ht->size; ++i)
    {
        ceil_value_t *ceil = ht->cells[i];

        if (ceil != NULL)
        {
            if (ceil->is_hash_table)
                free_perfect_table(ceil->data.hash_table);
            else
                free(ceil->data.pair.key);

            free(ceil);
        }
    }

    free(ht->cells);
    free(ht);
}
