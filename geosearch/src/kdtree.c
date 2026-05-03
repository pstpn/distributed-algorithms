#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <stdbool.h>

#include "kdtree.h"


static int8_t parse_line(char *line, double *x, double *y)
{
    if (!line || !x || !y)
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
    const char *y_str = comma + 1;

    char *end_ptr = NULL;
    const double parsed_y = strtod(y_str, &end_ptr);
    if (end_ptr == y_str || *end_ptr != '\0')
        return ERR_PARSE;
    const double parsed_x = strtod(line, &end_ptr);
    if (end_ptr == line || *end_ptr != '\0')
        return ERR_PARSE;

    *x = parsed_x;
    *y = parsed_y;

    return SUCCESS;
}

static int cmp_double(const void *a, const void *b)
{
    const double d = *(const double *)a - *(const double *)b;
    if (d < 0.0)
        return -1;
    if (d > 0.0)
        return 1;
    return 0;
}

static int cmp_point_x(const void *a, const void *b)
{
    const point_t *p1 = (const point_t *)a;
    const point_t *p2 = (const point_t *)b;
    return cmp_double(&p1->x, &p2->x);
}

static int cmp_point_y(const void *a, const void *b)
{
    const point_t *p1 = (const point_t *)a;
    const point_t *p2 = (const point_t *)b;
    return cmp_double(&p1->y, &p2->y);
}

static node_t *make_node(const point_t *point)
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

static void free_node(node_t *node)
{
    if (!node)
        return;

    free_node(node->left);
    free_node(node->right);

    free(node->point);
    free(node);
}

static node_t *build_tree(point_t *points, const int count, const bool split_by_x, int8_t *rc)
{
    if (count <= 0 || !points)
        return NULL;

    if (split_by_x)
        qsort(points, count, sizeof(point_t), cmp_point_x);
    else
        qsort(points, count, sizeof(point_t), cmp_point_y);

    const int mid = count / 2;
    node_t *root = make_node(&points[mid]);
    if (!root)
    {
        *rc = ERR_ALLOC;
        return NULL;
    }

    root->left = build_tree(points, mid, !split_by_x, rc);
    if (*rc)
    {
        free_node(root);
        return NULL;
    }

    root->right = build_tree(points + mid + 1, count - mid - 1, !split_by_x, rc);
    if (*rc)
    {
        free_node(root);
        return NULL;
    }

    return root;
}

static int8_t build_from_points(kdtree_t *kdtree, point_t *points, const int count)
{
    if (count <= 0)
        return SUCCESS;

    int8_t rc = SUCCESS;
    kdtree->root = build_tree(points, count, true, &rc);
    if (rc)
        return rc;

    kdtree->size = (size_t)count;
    return SUCCESS;
}

int8_t new_tree_from_csv(kdtree_t **kdtree, const char* path)
{
    if (!kdtree || !path)
        return ERR_EMPTY_INPUT;

    *kdtree = malloc(sizeof(kdtree_t));
    if (!*kdtree)
        return ERR_ALLOC;

    (*kdtree)->root = NULL;
    (*kdtree)->size = 0;

    FILE *file = fopen(path, "r");
    if (!file)
    {
        free(*kdtree);
        return ERR_OPEN_FILE;
    }

    char *line = NULL;
    size_t cap = 0;
    point_t *points = malloc(sizeof(point_t));
    size_t arr_cap = 1;
    size_t count = 0;

    if (!points)
    {
        fclose(file);
        free_tree(*kdtree);
        return ERR_ALLOC;
    }

    while (getline(&line, &cap, file) >= 0)
    {
        double point_x;
        double point_y;
        const int8_t rc = parse_line(line, &point_x, &point_y);
        if (rc)
        {
            free(line);
            free(points);
            fclose(file);
            free_tree(*kdtree);
            return rc;
        }

        if (count == arr_cap)
        {
            const size_t new_cap = arr_cap * 2;
            point_t *resized_points = realloc(points, new_cap * sizeof(point_t));
            if (!resized_points)
            {
                free(line);
                free(points);
                fclose(file);
                free_tree(*kdtree);
                return ERR_ALLOC;
            }

            points = resized_points;
            arr_cap = new_cap;
        }

        points[count].x = point_x;
        points[count].y = point_y;
        ++count;
    }

    free(line);
    fclose(file);

    const int8_t rc = build_from_points(*kdtree, points, (int)count);
    free(points);

    if (rc)
    {
        free_tree(*kdtree);
        return rc;
    }

    return SUCCESS;
}

static void insert_one(node_t *current_node, node_t *new_node, const bool is_x)
{
    if ((is_x && new_node->point->x <= current_node->point->x) ||
        (!is_x && new_node->point->y <= current_node->point->y))
        if (current_node->left)
            insert_one(current_node->left, new_node, !is_x);
        else
            current_node->left = new_node;
    else
        if (current_node->right)
            insert_one(current_node->right, new_node, !is_x);
        else
            current_node->right = new_node;
}

int8_t insert(kdtree_t *kdtree, node_t *node)
{
    if (!kdtree || !node || !node->point)
        return ERR_EMPTY_INPUT;

    node->left = NULL;
    node->right = NULL;

    if (!kdtree->root)
        kdtree->root = node;
    else
        insert_one(kdtree->root, node, true);

    ++kdtree->size;

    return SUCCESS;
}

static double distance_squared(const point_t *point, const double x, const double y)
{
    const double dx = point->x - x;
    const double dy = point->y - y;

    return dx * dx + dy * dy;
}

static void find_nearest_node(
    const node_t *node,
    const double x,
    const double y,
    const bool split_by_x,
    const node_t **best_node,
    double *best_distance
)
{
    if (!node)
        return;

    const double current_distance = distance_squared(node->point, x, y);
    if (current_distance < *best_distance)
    {
        *best_distance = current_distance;
        *best_node = node;
    }

    const double delta = split_by_x ? x - node->point->x : y - node->point->y;
    const node_t *near_branch = delta <= 0.0 ? node->left : node->right;
    const node_t *far_branch = delta <= 0.0 ? node->right : node->left;

    find_nearest_node(near_branch, x, y, !split_by_x, best_node, best_distance);

    if (delta * delta < *best_distance)
        find_nearest_node(far_branch, x, y, !split_by_x, best_node, best_distance);
}

int8_t find_nearest(const kdtree_t *kdtree, const double x, const double y, point_t *nearest_point)
{
    if (!kdtree || !kdtree->root || !nearest_point)
        return ERR_EMPTY_INPUT;

    const node_t *best_node = kdtree->root;
    double best_distance = distance_squared(kdtree->root->point, x, y);

    find_nearest_node(kdtree->root, x, y, true, &best_node, &best_distance);
    *nearest_point = *best_node->point;

    return SUCCESS;
}

void free_tree(kdtree_t *kdtree)
{
    if (!kdtree)
        return;

    free_node(kdtree->root);
    free(kdtree);
}
