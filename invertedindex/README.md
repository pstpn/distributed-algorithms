# Inverted index

A search index built from scratch, from the postings on disk up to a ranked query language,
and measured on collections from 500 to 200 000 documents.

## What it does

Queries support `AND`, `OR`, `NOT`, `ADJ` for adjacent terms and `NEAR/k` for terms within a
distance, with results ranked by BM25. A parser turns the query into an AST and an evaluator
walks it over the postings. There is a TUI for driving it by hand.

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
