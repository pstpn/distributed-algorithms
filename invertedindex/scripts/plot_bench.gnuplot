set datafile separator ","
set terminal pdfcairo enhanced color size 8in,5in font "Helvetica,11"
set key inside left top
set grid
set border linewidth 1.2
set pointsize 1.2
set logscale x 10
set logscale y 10
set autoscale xfix

set output plot_dir . "/build_time.pdf"
set title "Index build time"
set xlabel "Number of documents"
set ylabel "ns"
plot \
	raw_dir . "/build.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "build"

set output plot_dir . "/warmup_time.pdf"
set title "Warmup time"
set xlabel "Number of documents"
set ylabel "ns"
plot \
	raw_dir . "/warmup.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "warmup"

set output plot_dir . "/search_latency.pdf"
set title "Query latency"
set xlabel "Number of documents"
set ylabel "ns/query"
plot \
	raw_dir . "/search_Term.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "Term", \
	raw_dir . "/search_And.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "AND", \
	raw_dir . "/search_Or.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "OR", \
	raw_dir . "/search_Not.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "NOT", \
	raw_dir . "/search_Adj.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "ADJ", \
	raw_dir . "/search_Near.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "NEAR"

set output plot_dir . "/search_complex.pdf"
set title "Complex query latency"
set xlabel "Number of documents"
set ylabel "ns/query"
plot \
	raw_dir . "/search_Complex_AndOr.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "(a OR b) AND c", \
	raw_dir . "/search_Complex_AdjAnd.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "a ADJ b AND c", \
	raw_dir . "/search_Complex_AndNot.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "(a AND b) NOT c"

set output plot_dir . "/search_rank_latency.pdf"
set title "Query latency with ranking"
set xlabel "Number of documents"
set ylabel "ns/query"
plot \
	raw_dir . "/searchrank_Term.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "Term", \
	raw_dir . "/searchrank_And.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "AND", \
	raw_dir . "/searchrank_Or.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "OR", \
	raw_dir . "/searchrank_Not.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "NOT", \
	raw_dir . "/searchrank_Adj.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "ADJ", \
	raw_dir . "/searchrank_Near.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "NEAR"

set output plot_dir . "/search_rank_complex.pdf"
set title "Complex query latency with ranking"
set xlabel "Number of documents"
set ylabel "ns/query"
plot \
	raw_dir . "/searchrank_Complex_AndOr.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "(a OR b) AND c", \
	raw_dir . "/searchrank_Complex_AdjAnd.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "a ADJ b AND c", \
	raw_dir . "/searchrank_Complex_AndNot.csv" using 1:2:3 with yerrorlines linewidth 2 pt 7 title "(a AND b) NOT c"

unset logscale y
set output plot_dir . "/index_size.pdf"
set title "Index size"
set xlabel "Number of documents"
set ylabel "MB"
plot \
	raw_dir . "/index_size.csv" using 1:($2/1e6) with linespoints linewidth 2 pt 7 title "index"

set logscale y 10
