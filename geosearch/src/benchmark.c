#include <math.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "kdtree.h"

#define DEFAULT_BENCH_ITERS 100
#define DEFAULT_COUNT_PER_ITER 100


typedef struct
{
    point_t *points;
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
    if (!values || count <= 1)
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
    if (!ds)
        return;

    free(ds->points);
    ds->points = NULL;
    ds->size = 0;
}

static int8_t parse_line(char *line, double *out_x, double *out_y)
{
    if (!line || !out_x || !out_y)
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
    char *y_str = comma + 1;

    char *end_ptr = NULL;
    const double parsed_y = strtod(y_str, &end_ptr);
    if (end_ptr == y_str || *end_ptr != '\0')
        return ERR_PARSE;

    const double parsed_x = strtod(line, &end_ptr);
    if (end_ptr == line || *end_ptr != '\0')
        return ERR_PARSE;

    *out_x = parsed_x;
    *out_y = parsed_y;
    return SUCCESS;
}

static int8_t load_dataset(const char *path, dataset_t *out)
{
    FILE *f = fopen(path, "r");
    if (f == NULL)
        return ERR_OPEN_FILE;

    out->points = NULL;
    out->size = 0;

    size_t cap = 0;
    char *line = NULL;

    size_t used = 0;
    size_t arr_cap = 1024;
    out->points = malloc(sizeof(point_t) * arr_cap);
    if (out->points == NULL)
    {
        free(line);
        fclose(f);
        free_dataset(out);
        return ERR_ALLOC;
    }

    while (getline(&line, &cap, f) >= 0)
    {
        double point_x = 0.0;
        double point_y = 0.0;
        const int8_t rc = parse_line(line, &point_x, &point_y);
        if (rc != SUCCESS)
            continue;

        if (used == arr_cap)
        {
            arr_cap *= 2;
            point_t *new_points = realloc(out->points, sizeof(point_t) * arr_cap);
            if (!new_points)
            {
                free(line);
                fclose(f);
                free_dataset(out);
                return ERR_ALLOC;
            }

            out->points = new_points;
        }

        out->points[used].x = point_x;
        out->points[used].y = point_y;
        used++;
    }

    free(line);
    fclose(f);

    out->size = used;
    return SUCCESS;
}

static double random_in_range(uint32_t *state, const double min_v, const double max_v)
{
    *state = (*state * 1664525u) + 1013904223u;
    const double unit = (double)(*state) / (double)UINT32_MAX;
    return min_v + (max_v - min_v) * unit;
}

static node_t *make_insert_node(const point_t *point)
{
    node_t *node = malloc(sizeof(node_t));
    if (!node)
        return NULL;

    point_t *stored_point = malloc(sizeof(point_t));
    if (!stored_point)
    {
        free(node);
        return NULL;
    }

    *stored_point = *point;
    node->point = stored_point;
    node->left = NULL;
    node->right = NULL;
    return node;
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

    if (ds.size == 0)
    {
        free_dataset(&ds);
        fprintf(stderr, "dataset is empty: %s\n", path);
        return 1;
    }

    double min_x = ds.points[0].x;
    double max_x = ds.points[0].x;
    double min_y = ds.points[0].y;
    double max_y = ds.points[0].y;
    for (size_t i = 1; i < ds.size; ++i)
    {
        if (ds.points[i].x < min_x)
            min_x = ds.points[i].x;
        if (ds.points[i].x > max_x)
            max_x = ds.points[i].x;
        if (ds.points[i].y < min_y)
            min_y = ds.points[i].y;
        if (ds.points[i].y > max_y)
            max_y = ds.points[i].y;
    }

    int iters = DEFAULT_BENCH_ITERS;
    const char *iters_env = getenv("BENCH_ITERS");
    if (iters_env)
    {
        const int parsed = atoi(iters_env);
        if (parsed > 0)
            iters = parsed;
    }

    const size_t ops_per_iter = DEFAULT_COUNT_PER_ITER;

    double *insert_avg_samples = malloc(sizeof(double) * (size_t)iters);
    double *insert_thr_samples = malloc(sizeof(double) * (size_t)iters);
    double *search_avg_samples = malloc(sizeof(double) * (size_t)iters);
    double *search_thr_samples = malloc(sizeof(double) * (size_t)iters);
    if (!insert_avg_samples || !insert_thr_samples || !search_avg_samples || !search_thr_samples)
    {
        free(insert_avg_samples);
        free(insert_thr_samples);
        free(search_avg_samples);
        free(search_thr_samples);
        free_dataset(&ds);
        fprintf(stderr, "alloc failed for benchmark buffers\n");
        return 1;
    }

    double insert_sec_sum = 0.0;
    double insert_thr_sum = 0.0;
    volatile double sink = 0.0;

    for (int iter = 0; iter < iters; ++iter)
    {
        kdtree_t *insert_tree = NULL;
        rc = new_tree_from_csv(&insert_tree, path);
        if (rc != SUCCESS)
        {
            free(insert_avg_samples);
            free(insert_thr_samples);
            free(search_avg_samples);
            free(search_thr_samples);
            free_dataset(&ds);
            fprintf(stderr, "create insert tree failed for %s rc=%d\n", path, rc);
            return 1;
        }

        uint32_t rng_state = 0x9E3779B9u ^ (uint32_t)(iter + 1);

        const double t1 = now_sec();
        for (size_t i = 0; i < ops_per_iter; ++i)
        {
            point_t point;
            point.x = random_in_range(&rng_state, min_x, max_x);
            point.y = random_in_range(&rng_state, min_y, max_y);

            node_t *node = make_insert_node(&point);
            if (!node)
            {
                free_tree(insert_tree);
                free(insert_avg_samples);
                free(insert_thr_samples);
                free(search_avg_samples);
                free(search_thr_samples);
                free_dataset(&ds);
                fprintf(stderr, "alloc failed for insert node\n");
                return 1;
            }

            rc = insert(insert_tree, node);
            if (rc != SUCCESS)
            {
                free(node->point);
                free(node);
                free_tree(insert_tree);
                free(insert_avg_samples);
                free(insert_thr_samples);
                free(search_avg_samples);
                free(search_thr_samples);
                free_dataset(&ds);
                fprintf(stderr, "insert failed for %s at %zu rc=%d\n", path, i, rc);
                return 1;
            }
        }
        const double t2 = now_sec();

        const double insert_sec = t2 - t1;
        const double insert_thr = insert_sec > 0 ? (double)ops_per_iter / insert_sec : 0.0;

        insert_sec_sum += insert_sec;
        insert_thr_sum += insert_thr;
        insert_avg_samples[iter] = insert_sec / (double)ops_per_iter;
        insert_thr_samples[iter] = insert_thr;
        sink += (double)insert_tree->size;

        free_tree(insert_tree);
    }

    kdtree_t *tree = NULL;
    rc = new_tree_from_csv(&tree, path);
    if (rc != SUCCESS)
    {
        free(insert_avg_samples);
        free(insert_thr_samples);
        free(search_avg_samples);
        free(search_thr_samples);
        free_dataset(&ds);
        fprintf(stderr, "create kdtree failed for %s rc=%d\n", path, rc);
        return 1;
    }

    double search_sec_sum = 0.0;
    double search_thr_sum = 0.0;

    for (int iter = 0; iter < iters; ++iter)
    {
        const double t1 = now_sec();
        for (size_t i = 0; i < ops_per_iter; ++i)
        {
            const size_t idx = i % ds.size;
            point_t nearest = {0.0, 0.0};
            const double qx = ds.points[idx].x;
            const double qy = ds.points[idx].y;
            rc = find_nearest(tree, qx, qy, &nearest);
            if (rc != SUCCESS)
            {
                fprintf(stderr, "find_nearest failed for %s at %zu rc=%d\n", path, idx, rc);
                free_tree(tree);
                free(insert_avg_samples);
                free(insert_thr_samples);
                free(search_avg_samples);
                free(search_thr_samples);
                free_dataset(&ds);
                return 1;
            }

            sink += nearest.x + nearest.y;
        }
        const double t2 = now_sec();

        const double search_sec = t2 - t1;
        const double search_thr = search_sec > 0 ? (double)ops_per_iter / search_sec : 0.0;

        search_sec_sum += search_sec;
        search_thr_sum += search_thr;
        search_avg_samples[iter] = search_sec / (double)ops_per_iter;
        search_thr_samples[iter] = search_thr;
    }

    const double insert_avg_sec = insert_sec_sum / (double)iters / (double)ops_per_iter;
    const double insert_ci95_avg_sec = ci95_seconds(insert_avg_samples, iters);
    const double insert_ops_sec = insert_thr_sum / (double)iters;
    const double insert_ci95_ops_sec = ci95_seconds(insert_thr_samples, iters);

    const double search_avg_sec = search_sec_sum / (double)iters / (double)ops_per_iter;
    const double search_ci95_avg_sec = ci95_seconds(search_avg_samples, iters);
    const double search_ops_sec = search_thr_sum / (double)iters;
    const double search_ci95_ops_sec = ci95_seconds(search_thr_samples, iters);

    printf("%s,%zu,%d,%.12f,%.12f,%.2f,%.2f,%.12f,%.12f,%.2f,%.2f\n",
        path,
        ds.size,
        iters,
        insert_avg_sec,
        insert_ci95_avg_sec,
        insert_ops_sec,
        insert_ci95_ops_sec,
        search_avg_sec,
        search_ci95_avg_sec,
        search_ops_sec,
        search_ci95_ops_sec
    );

    if (sink == -1.0)
        fprintf(stderr, "sink=%f\n", sink);

    free_tree(tree);
    free(insert_avg_samples);
    free(insert_thr_samples);
    free(search_avg_samples);
    free(search_thr_samples);
    free_dataset(&ds);
    return 0;
}

int main(const int argc, char **argv)
{
    printf("file,rows,iterations,insert_avg_sec,insert_ci95_avg_sec,insert_ops_sec,insert_ci95_ops_sec,search_avg_sec,search_ci95_avg_sec,search_ops_sec,search_ci95_ops_sec\n");

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
