# Inverted index

A search index built from scratch, from the postings on disk up to a ranked query language,
and measured on collections from 500 to 200 000 documents.

## What it does

Queries support `AND`, `OR`, `NOT`, `ADJ` for adjacent terms and `NEAR/k` for terms within a
distance, with parentheses for grouping and results ranked by BM25. A parser turns the query
into an AST and an evaluator walks it over the postings.

## The interface

A terminal interface built on bubbletea drives all of it. The screens below are a real
session over 405 473 Wikipedia articles, indexed from the raw dumps.

| | |
|---|---|
| <img src="screenshots/menu.svg" width="470"> | <img src="screenshots/indexes.svg" width="470"> |
| Build an index or open one that exists | Indexes on disk, the largest holding 471 MB of postings |
| <img src="screenshots/results.svg" width="470"> | <img src="screenshots/document.svg" width="470"> |
| `MACHINE ADJ LEARNING AND (NEURAL OR DEEP) AND NOT QUANTUM` returns 57 documents, BM25 scores drawn as bars | Opening a hit shows the article that earned it |

The query in the third screen exercises every operator at once, and the document in the
fourth shows why it matched: the article carries the phrase *machine learning* next to *deep
neural network* and never mentions quantum anything.

## How it stores things

Posting lists are delta-coded and bitpacked into a binary format on disk, with skip lists so
that an intersection can jump over blocks instead of scanning them. The file is opened
through `mmap` and decoded lazily, so a query touches only the postings it needs.

<img src="plots/compression_ratio.svg" width="420"> <img src="plots/compression_size.svg" width="420">

*What delta coding and bitpacking take off the postings.*

<img src="plots/index_size.svg" width="420"> <img src="plots/build_time.svg" width="420">

*Index size and build time against the number of documents, both linear.*

## Search

<img src="plots/search_latency.svg" width="420"> <img src="plots/search_complex.svg" width="420">

*Latency by operator, simple and complex queries.*

<img src="plots/search_rank_latency.svg" width="420"> <img src="plots/search_rank_complex.svg" width="420">

*The same queries with BM25 ranking applied.*

<img src="plots/warmup_time.svg" width="420">

*The first query against the repeats.*

`Term` is the fastest operator and `Or` the slowest, which follows from the work each does:
a term lookup reads one list, while a union has to merge several and cannot skip. Warm-up
costs far more than any later search, since the first query is what faults the mapped pages
in.

## Profiles

<img src="plots/build_profile.svg" width="420"> <img src="plots/search_profile.svg" width="420">

*CPU profiles of building and searching.*
