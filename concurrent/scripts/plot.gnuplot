set datafile separator ","
set terminal pdfcairo enhanced color size 8in,5in font "Helvetica,11"
set key outside
set grid
set border linewidth 1.2
set tics out
set pointsize 1.1
set logscale x 10
set logscale y 10
set autoscale xfix

ci_linewidth = 3

set style line 1 linetype 1 linewidth 2 pointtype 1
set style line 2 linetype 2 linewidth 2 pointtype 2
set style line 3 linetype 3 linewidth 2 pointtype 3
set style line 4 linetype 4 linewidth 2 pointtype 4

if (!exists("input")) input = "data/benchmarks.csv"
if (!exists("outdir")) outdir = "results"

put_single_concurrent_lat = sprintf("< awk -F, -v op=put -v mode=single -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $5 \",\" $7}' %s", input)
put_single_baseline_lat = sprintf("< awk -F, -v op=put -v mode=single -v variant=baseline 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $5 \",\" $7}' %s", input)
get_single_concurrent_lat = sprintf("< awk -F, -v op=get -v mode=single -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $5 \",\" $7}' %s", input)
get_single_baseline_lat = sprintf("< awk -F, -v op=get -v mode=single -v variant=baseline 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $5 \",\" $7}' %s", input)

put_multi_concurrent_lat = sprintf("< awk -F, -v op=put -v mode=multi -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $5 \",\" $7}' %s", input)
put_multi_syncmap_lat = sprintf("< awk -F, -v op=put -v mode=multi -v variant=syncmap 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $5 \",\" $7}' %s", input)
get_multi_concurrent_lat = sprintf("< awk -F, -v op=get -v mode=multi -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $5 \",\" $7}' %s", input)
get_multi_syncmap_lat = sprintf("< awk -F, -v op=get -v mode=multi -v variant=syncmap 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $5 \",\" $7}' %s", input)

put_single_concurrent_thr = sprintf("< awk -F, -v op=put -v mode=single -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $6 \",\" $8}' %s", input)
put_single_baseline_thr = sprintf("< awk -F, -v op=put -v mode=single -v variant=baseline 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $6 \",\" $8}' %s", input)
get_single_concurrent_thr = sprintf("< awk -F, -v op=get -v mode=single -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $6 \",\" $8}' %s", input)
get_single_baseline_thr = sprintf("< awk -F, -v op=get -v mode=single -v variant=baseline 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $6 \",\" $8}' %s", input)

put_multi_concurrent_thr = sprintf("< awk -F, -v op=put -v mode=multi -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $6 \",\" $8}' %s", input)
put_multi_syncmap_thr = sprintf("< awk -F, -v op=put -v mode=multi -v variant=syncmap 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $6 \",\" $8}' %s", input)
get_multi_concurrent_thr = sprintf("< awk -F, -v op=get -v mode=multi -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $6 \",\" $8}' %s", input)
get_multi_syncmap_thr = sprintf("< awk -F, -v op=get -v mode=multi -v variant=syncmap 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $6 \",\" $8}' %s", input)

put_single_concurrent_mem = sprintf("< awk -F, -v op=put -v mode=single -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $9}' %s", input)
put_single_baseline_mem = sprintf("< awk -F, -v op=put -v mode=single -v variant=baseline 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $9}' %s", input)
get_single_concurrent_mem = sprintf("< awk -F, -v op=get -v mode=single -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $9}' %s", input)
get_single_baseline_mem = sprintf("< awk -F, -v op=get -v mode=single -v variant=baseline 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $9}' %s", input)

put_multi_concurrent_mem = sprintf("< awk -F, -v op=put -v mode=multi -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $9}' %s", input)
put_multi_syncmap_mem = sprintf("< awk -F, -v op=put -v mode=multi -v variant=syncmap 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $9}' %s", input)
get_multi_concurrent_mem = sprintf("< awk -F, -v op=get -v mode=multi -v variant=concurrent 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $9}' %s", input)
get_multi_syncmap_mem = sprintf("< awk -F, -v op=get -v mode=multi -v variant=syncmap 'NR>1 && $1==op && $2==mode && $3==variant {print $4 \",\" $9}' %s", input)

set xlabel "Keys count"
set ylabel "Latency (ns/op)"
set output outdir . "/latency_single.pdf"
set title "Latency (single thread)"
plot \
	put_single_concurrent_lat using 1:2 with linespoints ls 1 title "put (concurrent)", \
	put_single_concurrent_lat using 1:2:3 with yerrorbars ls 1 lw ci_linewidth pt 0 ps 0 notitle, \
	put_single_baseline_lat using 1:2 with linespoints ls 2 title "put (baseline)", \
	put_single_baseline_lat using 1:2:3 with yerrorbars ls 2 lw ci_linewidth pt 0 ps 0 notitle, \
	get_single_concurrent_lat using 1:2 with linespoints ls 3 title "get (concurrent)", \
	get_single_concurrent_lat using 1:2:3 with yerrorbars ls 3 lw ci_linewidth pt 0 ps 0 notitle, \
	get_single_baseline_lat using 1:2 with linespoints ls 4 title "get (baseline)", \
	get_single_baseline_lat using 1:2:3 with yerrorbars ls 4 lw ci_linewidth pt 0 ps 0 notitle

set output outdir . "/latency_multi.pdf"
set title "Latency (multi thread)"
plot \
	put_multi_concurrent_lat using 1:2 with linespoints ls 1 title "put (concurrent)", \
	put_multi_concurrent_lat using 1:2:3 with yerrorbars ls 1 lw ci_linewidth pt 0 ps 0 notitle, \
	put_multi_syncmap_lat using 1:2 with linespoints ls 2 title "put (syncmap)", \
	put_multi_syncmap_lat using 1:2:3 with yerrorbars ls 2 lw ci_linewidth pt 0 ps 0 notitle, \
	get_multi_concurrent_lat using 1:2 with linespoints ls 3 title "get (concurrent)", \
	get_multi_concurrent_lat using 1:2:3 with yerrorbars ls 3 lw ci_linewidth pt 0 ps 0 notitle, \
	get_multi_syncmap_lat using 1:2 with linespoints ls 4 title "get (syncmap)", \
	get_multi_syncmap_lat using 1:2:3 with yerrorbars ls 4 lw ci_linewidth pt 0 ps 0 notitle

set ylabel "Throughput (ops/sec)"
set output outdir . "/throughput_single.pdf"
set title "Throughput (single thread)"
plot \
	put_single_concurrent_thr using 1:2 with linespoints ls 1 title "put (concurrent)", \
	put_single_concurrent_thr using 1:2:3 with yerrorbars ls 1 lw ci_linewidth pt 0 ps 0 notitle, \
	put_single_baseline_thr using 1:2 with linespoints ls 2 title "put (baseline)", \
	put_single_baseline_thr using 1:2:3 with yerrorbars ls 2 lw ci_linewidth pt 0 ps 0 notitle, \
	get_single_concurrent_thr using 1:2 with linespoints ls 3 title "get (concurrent)", \
	get_single_concurrent_thr using 1:2:3 with yerrorbars ls 3 lw ci_linewidth pt 0 ps 0 notitle, \
	get_single_baseline_thr using 1:2 with linespoints ls 4 title "get (baseline)", \
	get_single_baseline_thr using 1:2:3 with yerrorbars ls 4 lw ci_linewidth pt 0 ps 0 notitle

set output outdir . "/throughput_multi.pdf"
set title "Throughput (multi thread)"
plot \
	put_multi_concurrent_thr using 1:2 with linespoints ls 1 title "put (concurrent)", \
	put_multi_concurrent_thr using 1:2:3 with yerrorbars ls 1 lw ci_linewidth pt 0 ps 0 notitle, \
	put_multi_syncmap_thr using 1:2 with linespoints ls 2 title "put (syncmap)", \
	put_multi_syncmap_thr using 1:2:3 with yerrorbars ls 2 lw ci_linewidth pt 0 ps 0 notitle, \
	get_multi_concurrent_thr using 1:2 with linespoints ls 3 title "get (concurrent)", \
	get_multi_concurrent_thr using 1:2:3 with yerrorbars ls 3 lw ci_linewidth pt 0 ps 0 notitle, \
	get_multi_syncmap_thr using 1:2 with linespoints ls 4 title "get (syncmap)", \
	get_multi_syncmap_thr using 1:2:3 with yerrorbars ls 4 lw ci_linewidth pt 0 ps 0 notitle

set ylabel "Heap bytes per op"
set output outdir . "/memory_single.pdf"
set title "Memory per operation (single thread)"
plot \
	put_single_concurrent_mem using 1:2 with linespoints ls 1 title "put (concurrent)", \
	put_single_baseline_mem using 1:2 with linespoints ls 2 title "put (baseline)", \
	get_single_concurrent_mem using 1:2 with linespoints ls 3 title "get (concurrent)", \
	get_single_baseline_mem using 1:2 with linespoints ls 4 title "get (baseline)"

set output outdir . "/memory_multi.pdf"
set title "Memory per operation (multi thread)"
plot \
	put_multi_concurrent_mem using 1:2 with linespoints ls 1 title "put (concurrent)", \
	put_multi_syncmap_mem using 1:2 with linespoints ls 2 title "put (syncmap)", \
	get_multi_concurrent_mem using 1:2 with linespoints ls 3 title "get (concurrent)", \
	get_multi_syncmap_mem using 1:2 with linespoints ls 4 title "get (syncmap)"

