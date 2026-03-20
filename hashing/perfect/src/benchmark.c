#include <math.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "hash.h"
#include "perfect.h"

#define DEFAULT_BENCH_ITERS 100
#define DEFAULT_COUNT_PER_ITER 100000


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

static double ci95_seconds(const double *values, int count)
{
    if (values == NULL || count <= 1)
        return 0.0;

    double mean = 0.0;
    for (int i = 0; i < count; ++i)
        mean += values[i];
    mean /= (double)count;

    double variance = 0.0;
    for (int i = 0; i < count; ++i)
    {
        const double d = values[i] - mean;
        variance += d * d;
    }
    variance /= (double)(count - 1);

    const double stderr = sqrt(variance / (double)count);
    return 1.96 * stderr;
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
    if (!comma || comma[1] == '\0')
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
            if (!new_keys)
            {
                free(line);
                fclose(f);
                free_dataset(out);
                return ERR_ALLOC;
            }

            out->keys = new_keys;

            double *new_vals = realloc(out->values, sizeof(double) * arr_cap);
            if (new_vals == NULL)
            {
                free(line);
                fclose(f);
                free_dataset(out);
                return ERR_ALLOC;
            }
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

    double get_sec_sum = 0.0;
    double get_thr_sum = 0.0;
    double *get_avg_samples = malloc(sizeof(double) * (size_t)iters);
    if (!get_avg_samples)
    {
        free(get_avg_samples);
        free_dataset(&ds);
        fprintf(stderr, "alloc failed for benchmark buffers\n");
        return 1;
    }

    perfect_hash_table_t *ht = NULL;
    rc = perfect_table_from_csv(path, hash_mad_a, &ht);
    if (rc != SUCCESS)
    {
        free(get_avg_samples);
        free_dataset(&ds);
        fprintf(stderr, "create static table failed for %s rc=%d\n", path, rc);
        return 1;
    }

    for (int iter = 0; iter < iters; ++iter)
    {
        const double t1 = now_sec();
        for (size_t i = 0; i < DEFAULT_COUNT_PER_ITER; ++i)
        {
            const size_t idx = i % ds.size;
            double out = 0.0;
            rc = perfect_get(ht, ds.keys[idx], &out);
            if (rc != SUCCESS || fabs(out - ds.values[idx]) > 1e-9)
            {
                fprintf(stderr, "get failed for %s at %zu rc=%d\n", path, idx, rc);
                free_perfect_table(ht);
                free(get_avg_samples);
                free_dataset(&ds);
                return 1;
            }
        }
        const double t2 = now_sec();

        const double get_sec = t2 - t1;
        const double get_thr = get_sec > 0 ? (double)DEFAULT_COUNT_PER_ITER / get_sec : 0.0;

        get_sec_sum += get_sec;
        get_thr_sum += get_thr;
        get_avg_samples[iter] = get_sec / (double)DEFAULT_COUNT_PER_ITER;
    }

    const double get_sec = get_sec_sum / (double)iters;
    const double get_avg_sec =  get_sec / (double)DEFAULT_COUNT_PER_ITER;
    const double get_ci95_avg_sec = ci95_seconds(get_avg_samples, iters);
    const double get_thr = get_thr_sum / (double)iters;

    printf("%s,%zu,%d,%.9f,%.12f,%.12f,%.2f\n",
        path,
        ds.size,
        iters,
        get_sec,
        get_avg_sec,
        get_ci95_avg_sec,
        get_thr);

    free_perfect_table(ht);
    free(get_avg_samples);
    free_dataset(&ds);
    return 0;
}

int main(int argc, char **argv)
{
    printf("file,rows,iterations,get_sec,get_avg_sec,get_ci95_avg_sec,get_ops_sec\n");

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
