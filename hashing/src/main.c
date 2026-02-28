#include <stdio.h>

#include "perfect.h"
#include "hash.h"


int main(const int argc, char **argv)
{
    if (argc < 2)
        return ERR_EMPTY_INPUT;

    perfect_hash_table_t *ht;
    int8_t rc = perfect_table_from_csv(argv[1], mad_hash1, &ht);
    if (rc != SUCCESS)
        return rc;

    const char *k = "i am english i am i do not want to get blocked again fu k";
    double val;
    rc = perfect_get(ht, k, &val);
    if (rc != SUCCESS)
        return rc;

    printf("%s: %lf\n", k, val);

    free_perfect_table(ht);

    return 0;
}
