#ifndef __KDTREE_H__
#define __KDTREE_H__

#include <stddef.h>
#include <stdint.h>


enum RC
{
    SUCCESS,
    ERR_ALLOC,
    ERR_PARSE,
    ERR_OPEN_FILE,
    ERR_EMPTY_INPUT
};

typedef struct point
{
    double x, y;
} point_t;

typedef struct node
{
    point_t *point;
    struct node *left;
    struct node *right;
} node_t;

typedef struct kdtree
{
    node_t *root;
    size_t size;
} kdtree_t;

int8_t new_tree_from_csv(kdtree_t **, const char *);
int8_t insert(kdtree_t *, node_t *);
int8_t find_nearest(const kdtree_t *, double, double, point_t *);
void free_tree(kdtree_t *);

#endif //__KDTREE_H__
