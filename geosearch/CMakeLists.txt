cmake_minimum_required(VERSION 3.31)
project(hashing C)

set(CMAKE_C_STANDARD 11)

include_directories(inc)

add_executable(geosearch
        inc/kdtree.h
        src/kdtree.c
)
