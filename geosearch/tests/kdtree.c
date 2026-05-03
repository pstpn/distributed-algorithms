#include <check.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <unistd.h>
#include <math.h>
#include <string.h>

#include "kdtree.h"


static int make_temp_csv(char *path_buf, const size_t path_buf_size, const char *content)
{
    if (!path_buf || path_buf_size < 32 || !content)
        return -1;

    snprintf(path_buf, path_buf_size, "/tmp/kdtree_test_%ld_XXXXXX.csv", time(NULL));
    const int fd = mkstemps(path_buf, 4);
    if (fd < 0)
        return -1;

    FILE *file = fdopen(fd, "w");
    if (!file)
    {
        close(fd);
        unlink(path_buf);
        return -1;
    }

    if (fputs(content, file) < 0)
    {
        fclose(file);
        unlink(path_buf);
        return -1;
    }

    fclose(file);
    return 0;
}

START_TEST(test_new_tree_from_csv_valid)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "1.0,2.0\n3.5,4.5\n-1.0,-2.0\n"), 0);

    kdtree_t *tree;
    const int8_t rc = new_tree_from_csv(&tree, path);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_ptr_nonnull(tree);
    ck_assert_ptr_nonnull(tree->root);
    ck_assert_uint_eq(tree->size, 3);

    free_tree(tree);
    unlink(path);
}
END_TEST

START_TEST(test_new_tree_from_csv_null_args)
{
    kdtree_t *tree;
    ck_assert_int_eq(new_tree_from_csv(NULL, "/tmp/test.csv"), ERR_EMPTY_INPUT);
    ck_assert_int_eq(new_tree_from_csv(&tree, NULL), ERR_EMPTY_INPUT);
}
END_TEST

START_TEST(test_new_tree_from_csv_missing_file)
{
    kdtree_t *tree;
    const int8_t rc = new_tree_from_csv(&tree, "/tmp/definitely_missing_kdtree_file.csv");
    ck_assert_int_eq(rc, ERR_OPEN_FILE);
}
END_TEST

START_TEST(test_new_tree_from_csv_invalid_line)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "1.0,2.0\nbad_line\n"), 0);

    kdtree_t *tree;
    const int8_t rc = new_tree_from_csv(&tree, path);
    ck_assert_int_eq(rc, ERR_PARSE);

    unlink(path);
}
END_TEST

START_TEST(test_new_tree_from_csv_missing_comma)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "1.0,2.0\n3.5\n"), 0);

    kdtree_t *tree;
    const int8_t rc = new_tree_from_csv(&tree, path);
    ck_assert_int_eq(rc, ERR_PARSE);

    unlink(path);
}
END_TEST

START_TEST(test_new_tree_from_csv_single_point)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "5.0,10.0\n"), 0);

    kdtree_t *tree;
    const int8_t rc = new_tree_from_csv(&tree, path);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_ptr_nonnull(tree);
    ck_assert_ptr_nonnull(tree->root);
    ck_assert_uint_eq(tree->size, 1);

    free_tree(tree);
    unlink(path);
}
END_TEST

START_TEST(test_find_nearest_exact_match)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "0.0,0.0\n1.0,1.0\n2.0,2.0\n"), 0);

    kdtree_t *tree;
    ck_assert_int_eq(new_tree_from_csv(&tree, path), SUCCESS);

    point_t nearest = {0, 0};
    const int8_t rc = find_nearest(tree, 1.0, 1.0, &nearest);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_double_eq(nearest.x, 1.0);
    ck_assert_double_eq(nearest.y, 1.0);

    free_tree(tree);
    unlink(path);
}
END_TEST

START_TEST(test_find_nearest_closest)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "0.0,0.0\n5.0,5.0\n10.0,10.0\n"), 0);

    kdtree_t *tree;
    ck_assert_int_eq(new_tree_from_csv(&tree, path), SUCCESS);

    point_t nearest = {0, 0};
    const int8_t rc = find_nearest(tree, 4.9, 4.9, &nearest);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_double_eq(nearest.x, 5.0);
    ck_assert_double_eq(nearest.y, 5.0);

    free_tree(tree);
    unlink(path);
}
END_TEST

START_TEST(test_find_nearest_null_args)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "1.0,2.0\n"), 0);

    kdtree_t *tree;
    ck_assert_int_eq(new_tree_from_csv(&tree, path), SUCCESS);

    point_t nearest = {0, 0};
    ck_assert_int_eq(find_nearest(NULL, 0.0, 0.0, &nearest), ERR_EMPTY_INPUT);
    ck_assert_int_eq(find_nearest(tree, 0.0, 0.0, NULL), ERR_EMPTY_INPUT);

    free_tree(tree);
    unlink(path);
}
END_TEST

START_TEST(test_insert_node)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "0.0,0.0\n2.0,2.0\n"), 0);

    kdtree_t *tree;
    ck_assert_int_eq(new_tree_from_csv(&tree, path), SUCCESS);
    ck_assert_uint_eq(tree->size, 2);

    const point_t new_point = {1.0, 1.0};
    node_t *new_node = malloc(sizeof(node_t));
    ck_assert_ptr_nonnull(new_node);
    new_node->point = malloc(sizeof(point_t));
    ck_assert_ptr_nonnull(new_node->point);
    *new_node->point = new_point;
    new_node->left = NULL;
    new_node->right = NULL;

    const int8_t rc = insert(tree, new_node);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_uint_eq(tree->size, 3);

    point_t found = {0, 0};
    ck_assert_int_eq(find_nearest(tree, 1.0, 1.0, &found), SUCCESS);
    ck_assert_double_eq(found.x, 1.0);
    ck_assert_double_eq(found.y, 1.0);

    free_tree(tree);
    unlink(path);
}
END_TEST

START_TEST(test_insert_null_args)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "1.0,1.0\n"), 0);

    kdtree_t *tree;
    ck_assert_int_eq(new_tree_from_csv(&tree, path), SUCCESS);

    point_t point = {2.0, 2.0};
    node_t *node = malloc(sizeof(node_t));
    node->point = malloc(sizeof(point_t));
    *node->point = point;
    node->left = NULL;
    node->right = NULL;

    ck_assert_int_eq(insert(NULL, node), ERR_EMPTY_INPUT);

    free(node->point);
    free(node);
    free_tree(tree);
    unlink(path);
}
END_TEST

START_TEST(test_large_dataset)
{
    char path[128] = {0};
    FILE *csv = fdopen(mkstemps(strcpy(path, "/tmp/kdtree_large_XXXXXX.csv"), 4), "w");
    ck_assert_ptr_nonnull(csv);

    for (int i = 0; i < 100; ++i)
        fprintf(csv, "%.1f,%.1f\n", (double)i * 0.5, (double)i * 0.3);
    fclose(csv);

    kdtree_t *tree;
    const int8_t rc = new_tree_from_csv(&tree, path);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_ptr_nonnull(tree);
    ck_assert_uint_eq(tree->size, 100);

    point_t nearest = {0, 0};
    ck_assert_int_eq(find_nearest(tree, 25.0, 15.0, &nearest), SUCCESS);
    ck_assert_double_ge(nearest.x, 0.0);
    ck_assert_double_ge(nearest.y, 0.0);

    free_tree(tree);
    unlink(path);
}
END_TEST

Suite *kdtree_suite(void)
{
    Suite *s = suite_create("kdtree");
    TCase *tcase = tcase_create("kdtree");

    tcase_add_test(tcase, test_new_tree_from_csv_valid);
    tcase_add_test(tcase, test_new_tree_from_csv_null_args);
    tcase_add_test(tcase, test_new_tree_from_csv_missing_file);
    tcase_add_test(tcase, test_new_tree_from_csv_invalid_line);
    tcase_add_test(tcase, test_new_tree_from_csv_missing_comma);
    tcase_add_test(tcase, test_new_tree_from_csv_single_point);
    tcase_add_test(tcase, test_find_nearest_exact_match);
    tcase_add_test(tcase, test_find_nearest_closest);
    tcase_add_test(tcase, test_find_nearest_null_args);
    tcase_add_test(tcase, test_insert_node);
    tcase_add_test(tcase, test_insert_null_args);
    tcase_add_test(tcase, test_large_dataset);

    suite_add_tcase(s, tcase);

    return s;
}

int main(void)
{
    srand((unsigned int)time(NULL));
    Suite *s = kdtree_suite();
    SRunner *sr = srunner_create(s);

    srunner_run_all(sr, CK_VERBOSE);
    const int number_failed = srunner_ntests_failed(sr);
    srunner_free(sr);

    return !number_failed ? EXIT_SUCCESS : EXIT_FAILURE;
}
