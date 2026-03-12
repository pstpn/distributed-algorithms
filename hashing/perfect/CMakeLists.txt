cmake_minimum_required(VERSION 3.31)
project(hashing C)

set(CMAKE_C_STANDARD 11)

include_directories(inc)

add_executable(hashing
        inc/perfect.h
        src/perfect.c
        src/main.c
        src/hash.c
        inc/hash.h
        inc/primes.h
        src/primes.c
)
