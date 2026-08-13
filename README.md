# Algorithms for high-load systems

Five studies, each implementing a data structure that a high-load system leans on, then
measuring it against the obvious alternative rather than trusting the asymptotics. Every one
carries its own report with the method, the hardware, the tables and the plots.

| Study | Structure | Measured against |
|-------|-----------|------------------|
| [Concurrent map](concurrent) | Hash table with striped locking | `map[string]string` and `sync.Map` |
| [Geosearch](geosearch) | kd-tree over coordinates | Insert against nearest-neighbour search as data grows |
| [Hashing](hashing) | Perfect, extendible and MinHash LSH | Each other, and a full scan as the baseline |
| [Inverted index](invertedindex) | Postings with skip lists, bitpacking and BM25 | Query operators against each other |
| [Vector search](vectorsearch) | Approximate nearest neighbours | Notebook experiment |

Measurements everywhere are averaged over repeated rounds, and the confidence intervals are
reported where the spread matters.
