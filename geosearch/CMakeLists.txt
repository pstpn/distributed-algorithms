cmake_minimum_required(VERSION 3.31)
project(geosearch C)

set(CMAKE_C_STANDARD 11)

include_directories(${CMAKE_CURRENT_SOURCE_DIR}/inc)

add_executable(geosearch
        src/kdtree.c
        src/main.c
)
