#include <stdlib.h>
#include <stdio.h>

#include "kdtree.h"


int main(void)
{
    kdtree_t *kdtree;
    int8_t rc = new_tree_from_csv(&kdtree, "./data/bench/train_data_bench_1000000.csv");
    if (rc)
        return rc;
    
    printf("size: %zu\n", kdtree->size);

    // point_t nearest = {0, 0};
    // find_nearest(kdtree, 13, -13, &nearest);
    // printf("nearest: (%f, %f)\n", nearest.x, nearest.y);

    free_tree(kdtree);

    return SUCCESS;
}