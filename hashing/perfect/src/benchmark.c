#include <math.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "hash.h"
#include "perfect.h"

#define DEFAULT_BENCH_ITERS 5


typedef struct
{
    char **keys;
    double *values;
    size_t size;
} dataset_t;

static double now_sec(void)
{
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (double)ts.tv_sec + (double)ts.tv_nsec / 1e9;
}

static void free_dataset(dataset_t *ds)
{
    if (ds == NULL)
        return;

    if (ds->keys != NULL)
    {
        for (size_t i = 0; i < ds->size; ++i)
            free(ds->keys[i]);
    }

    free(ds->keys);
    free(ds->values);

    ds->keys = NULL;
    ds->values = NULL;
    ds->size = 0;
}

static int8_t parse_line(char *line, char **out_text, double *out_value)
{
    if (line == NULL || out_text == NULL || out_value == NULL)
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
    char *value_str = comma + 1;

    char *end_ptr = NULL;
    double value = strtod(value_str, &end_ptr);
    if (end_ptr == value_str || *end_ptr != '\0')
        return ERR_PARSE;

    *out_text = line;
    *out_value = value;
    return SUCCESS;
}

static int8_t load_dataset(const char *path, dataset_t *out)
{
    FILE *f = fopen(path, "r");
    if (f == NULL)
        return ERR_OPEN_FILE;

    out->keys = NULL;
    out->values = NULL;
    out->size = 0;

    size_t cap = 0;
    char *line = NULL;

    if (getline(&line, &cap, f) < 0)
    {
        free(line);
        fclose(f);
        return ERR_PARSE;
    }

    size_t used = 0;
    size_t arr_cap = 1024;
    out->keys = malloc(sizeof(char *) * arr_cap);
    out->values = malloc(sizeof(double) * arr_cap);
    if (out->keys == NULL || out->values == NULL)
    {
        free(line);
        fclose(f);
        free_dataset(out);
        return ERR_ALLOC;
    }

    while (getline(&line, &cap, f) >= 0)
    {
        char *text = NULL;
        double val = 0.0;
        int8_t rc = parse_line(line, &text, &val);
        if (rc != SUCCESS)
            continue;

        if (used == arr_cap)
        {
            arr_cap *= 2;
            char **new_keys = realloc(out->keys, sizeof(char *) * arr_cap);
            double *new_vals = realloc(out->values, sizeof(double) * arr_cap);
            if (new_keys == NULL || new_vals == NULL)
            {
                free(new_keys);
                free(new_vals);
                free(line);
                fclose(f);
                free_dataset(out);
                return ERR_ALLOC;
            }
            out->keys = new_keys;
            out->values = new_vals;
        }

        out->keys[used] = strdup(text);
        if (out->keys[used] == NULL)
        {
            free(line);
            fclose(f);
            out->size = used;
            free_dataset(out);
            return ERR_ALLOC;
        }

        out->values[used] = val;
        used++;
    }

    free(line);
    fclose(f);

    out->size = used;
    return SUCCESS;
}

static int benchmark_one(const char *path)
{
    dataset_t ds;
    int8_t rc = load_dataset(path, &ds);
    if (rc != SUCCESS)
    {
        fprintf(stderr, "load failed for %s rc=%d\n", path, rc);
        return 1;
    }

    int iters = DEFAULT_BENCH_ITERS;
    const char *iters_env = getenv("BENCH_ITERS");
    if (iters_env)
    {
        int parsed = atoi(iters_env);
        if (parsed > 0)
            iters = parsed;
    }

    double insert_sec_sum = 0.0;
    double get_sec_sum = 0.0;
    double ins_thr_sum = 0.0;
    double get_thr_sum = 0.0;

    for (int iter = 0; iter < iters; ++iter)
    {
        perfect_hash_table_t *ht = NULL;
        rc = create_perfect_table(hash_mad_a, ds.size, &ht);
        if (rc != SUCCESS)
        {
            free_dataset(&ds);
            fprintf(stderr, "create table failed for %s rc=%d\n", path, rc);
            return 1;
        }

        double t0 = now_sec();
        for (size_t i = 0; i < ds.size; ++i)
        {
            rc = perfect_set(ht, ds.keys[i], ds.values[i]);
            if (rc != SUCCESS)
            {
                fprintf(stderr, "insert failed for %s at %zu rc=%d\n", path, i, rc);
                free_perfect_table(ht);
                free_dataset(&ds);
                return 1;
            }
        }
        double t1 = now_sec();

        for (size_t i = 0; i < ds.size; ++i)
        {
            double out = 0.0;
            rc = perfect_get(ht, ds.keys[i], &out);
            if (rc != SUCCESS || fabs(out - ds.values[i]) > 1e-9)
            {
                fprintf(stderr, "get failed for %s at %zu rc=%d\n", path, i, rc);
                free_perfect_table(ht);
                free_dataset(&ds);
                return 1;
            }
        }
        double t2 = now_sec();

        const double insert_sec = t1 - t0;
        const double get_sec = t2 - t1;
        const double ins_thr = insert_sec > 0 ? (double)ds.size / insert_sec : 0.0;
        const double get_thr = get_sec > 0 ? (double)ds.size / get_sec : 0.0;

        insert_sec_sum += insert_sec;
        get_sec_sum += get_sec;
        ins_thr_sum += ins_thr;
        get_thr_sum += get_thr;

        free_perfect_table(ht);
    }

    double insert_sec = insert_sec_sum / (double)iters;
    double get_sec = get_sec_sum / (double)iters;
    double insert_avg_sec = ds.size > 0 ? insert_sec / (double)ds.size : 0.0;
    double get_avg_sec = ds.size > 0 ? get_sec / (double)ds.size : 0.0;
    double ins_thr = ins_thr_sum / (double)iters;
    double get_thr = get_thr_sum / (double)iters;

    printf("%s,%zu,%.9f,%.12f,%.2f,%.9f,%.12f,%.2f\n",
        path,
        ds.size,
        insert_sec,
        insert_avg_sec,
        ins_thr,
        get_sec,
        get_avg_sec,
        get_thr);

    free_dataset(&ds);
    return 0;
}

int main(int argc, char **argv)
{
    printf("file,rows,insert_sec,insert_avg_sec,insert_ops_sec,get_sec,get_avg_sec,get_ops_sec\n");

    if (argc > 1)
    {
        for (int i = 1; i < argc; ++i)
            if (benchmark_one(argv[i]) != 0)
                return 1;
        return 0;
    }

    FILE *manifest = fopen("data/bench/manifest.txt", "r");
    if (manifest == NULL)
    {
        fprintf(stderr, "manifest not found: data/bench/manifest.txt\n");
        return 1;
    }

    char *line = NULL;
    size_t cap = 0;
    while (getline(&line, &cap, manifest) >= 0)
    {
        size_t len = strlen(line);
        while (len > 0 && (line[len - 1] == '\n' || line[len - 1] == '\r'))
            line[--len] = '\0';

        if (line[0] == '\0')
            continue;

        if (benchmark_one(line) != 0)
        {
            free(line);
            fclose(manifest);
            return 1;
        }
    }

    free(line);
    fclose(manifest);
    return 0;
}
