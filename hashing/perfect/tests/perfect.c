#include <check.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <unistd.h>

#include "perfect.h"


static size_t table_max_depth(const perfect_hash_table_t *table)
{
    if (!table)
        return 0;

    size_t max_child_depth = 0;
    for (size_t i = 0; i < table->size; ++i)
    {
        const ceil_value_t *cell = table->cells[i];
        if (cell && cell->is_hash_table)
        {
            const size_t child_depth = table_max_depth(cell->data.hash_table);
            if (child_depth > max_child_depth)
                max_child_depth = child_depth;
        }
    }

    return 1 + max_child_depth;
}

static void assert_depth_leq(const perfect_hash_table_t *table, const size_t max_depth)
{
    const size_t depth = table_max_depth(table);
    ck_assert_msg(depth <= max_depth, "Nesting depth is %zu, expected <= %zu", depth, max_depth);
}

static int make_temp_csv(char *path_buf, const size_t path_buf_size, const char *content)
{
    if (!path_buf || path_buf_size < 32 || !content)
        return -1;

    snprintf(path_buf, path_buf_size, "/tmp/perfect_test_%ld_XXXXXX.csv", time(NULL));
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

START_TEST(test_create_free)
{
    perfect_hash_table_t *table;
    const int8_t rc = create_perfect_table(NULL, 100, &table);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_ptr_nonnull(table);
    assert_depth_leq(table, 1);

    free_perfect_table(table);
}
END_TEST

START_TEST(test_get_nonexistent)
{
    perfect_hash_table_t *table;
    const int8_t rc = create_perfect_table(NULL, 100, &table);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_ptr_nonnull(table);

    double out_val = 0;
    ck_assert_int_eq(perfect_get(table, "nothing", &out_val), ERR_NOT_FOUND);
    assert_depth_leq(table, 1);

    free_perfect_table(table);
}
END_TEST

START_TEST(test_create_zero_size)
{
    perfect_hash_table_t *table;
    const int8_t rc = create_perfect_table(NULL, 0, &table);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_ptr_nonnull(table);
    ck_assert_uint_ge(table->size, 2);
    assert_depth_leq(table, 1);

    free_perfect_table(table);
}
END_TEST

START_TEST(test_get_null_value_ptr)
{
    perfect_hash_table_t *table;
    const int8_t rc = create_perfect_table(NULL, 8, &table);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_ptr_nonnull(table);

    ck_assert_int_eq(perfect_get(table, "k", NULL), ERR_EMPTY_INPUT);
    assert_depth_leq(table, 1);

    free_perfect_table(table);
}
END_TEST

START_TEST(test_csv_null_args)
{
    perfect_hash_table_t *table = NULL;
    ck_assert_int_eq(perfect_table_from_csv(NULL, NULL, &table), ERR_EMPTY_INPUT);
    ck_assert_int_eq(perfect_table_from_csv("data/train_data_version1.csv", NULL, NULL), ERR_EMPTY_INPUT);
    ck_assert_ptr_null(table);
}
END_TEST

START_TEST(test_csv_missing_file)
{
    perfect_hash_table_t *table = NULL;
    ck_assert_int_eq(perfect_table_from_csv("/tmp/definitely_missing_file.csv", NULL, &table), ERR_OPEN_FILE);
    ck_assert_ptr_null(table);
}
END_TEST

START_TEST(test_csv_invalid_line)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "text,y\nok,1.5\nbad_line_without_comma\n"), 0);

    perfect_hash_table_t *table = NULL;
    const int8_t rc = perfect_table_from_csv(path, NULL, &table);
    ck_assert_int_eq(rc, ERR_PARSE);
    ck_assert_ptr_null(table);

    unlink(path);
}
END_TEST

START_TEST(test_csv_valid_small)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "text,y\na,1.25\nb,2.5\n"), 0);

    perfect_hash_table_t *table = NULL;
    const int8_t rc = perfect_table_from_csv(path, NULL, &table);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_ptr_nonnull(table);

    double out_val;
    ck_assert_int_eq(perfect_get(table, "a", &out_val), SUCCESS);
    ck_assert_double_eq(out_val, 1.25);
    ck_assert_int_eq(perfect_get(table, "b", &out_val), SUCCESS);
    ck_assert_double_eq(out_val, 2.5);
    assert_depth_leq(table, 2);

    unlink(path);
    free_perfect_table(table);
}
END_TEST

START_TEST(test_csv_empty_key)
{
    char path[128] = {0};
    ck_assert_int_eq(make_temp_csv(path, sizeof(path), "text,y\n,42.25\n"), 0);

    perfect_hash_table_t *table = NULL;
    const int8_t rc = perfect_table_from_csv(path, NULL, &table);
    ck_assert_int_eq(rc, SUCCESS);
    ck_assert_ptr_nonnull(table);

    double out_val;
    ck_assert_int_eq(perfect_get(table, "", &out_val), SUCCESS);
    ck_assert_double_eq(out_val, 42.25);
    assert_depth_leq(table, 2);

    unlink(path);
    free_perfect_table(table);
}
END_TEST

Suite *hashing_suite(void)
{
    Suite *s = suite_create("perfect_hashing");
    TCase *tcase = tcase_create("perfect_hashing");

    tcase_add_test(tcase, test_create_free);
    tcase_add_test(tcase, test_get_nonexistent);
    tcase_add_test(tcase, test_create_zero_size);
    tcase_add_test(tcase, test_get_null_value_ptr);
    tcase_add_test(tcase, test_csv_null_args);
    tcase_add_test(tcase, test_csv_missing_file);
    tcase_add_test(tcase, test_csv_invalid_line);
    tcase_add_test(tcase, test_csv_valid_small);
    tcase_add_test(tcase, test_csv_empty_key);

    suite_add_tcase(s, tcase);

    return s;
}

int main(void)
{
    srand((unsigned int)time(NULL));
    Suite *s = hashing_suite();
    SRunner *sr = srunner_create(s);

    srunner_run_all(sr, CK_VERBOSE);
    const int number_failed = srunner_ntests_failed(sr);
    srunner_free(sr);

    return (!number_failed) ? EXIT_SUCCESS : EXIT_FAILURE;
}
