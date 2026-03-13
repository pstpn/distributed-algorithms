# Extendible Hashing in Go

В каталоге реализована чистая in-memory реализация extendible hashing на Go с операциями:

- insert
- update
- delete
- get
- put (upsert)

Текущая версия использует файловую модель хранения с mmap и прямым доступом к данным на диске.
Служебный заголовок, директория и бакеты читаются и обновляются напрямую в отображенной памяти файла, без сериализации и десериализации всей таблицы в отдельные структуры обменного формата.

Минимальная версия Go: 1.25.

## Что входит

- реализация структуры данных в [extendible.go](extendible.go)
- property-based и fuzz tests в [extendible_test.go](extendible_test.go)
- benchmark tests для insert/update/delete в [benchmark_test.go](benchmark_test.go)
	- в бенчмарках используется новый цикл `b.Loop()` из Go 1.25
- CPU/heap profiling CLI в [cmd/profile/main.go](cmd/profile/main.go)
- сборщик метрик и генератор PDF-графиков в [cmd/collectmetrics/main.go](cmd/collectmetrics/main.go)
- gnuplot-скрипт для визуализации в [scripts/plot_metrics.gnuplot](scripts/plot_metrics.gnuplot)

## Быстрый старт

```bash
go test ./...
go test -run '^$' -bench '^BenchmarkTable' -benchmem .
go run ./cmd/profile -op insert -size 100000 -out metrics/profiles
go run ./cmd/collectmetrics -sizes 10000,100000,1000000 -out metrics
```

После запуска сборщика метрик артефакты будут лежать в каталоге `metrics`:

- `metrics/raw/benchmarks.txt` и `metrics/raw/benchmarks.csv`
- `metrics/raw/profiles.csv`
- `metrics/raw/profiles/*.pprof`
- `metrics/plots/*.pdf`

## Замечания по реализации

- Директория удваивается только при необходимости split.
- При delete выполняется merge buddy-бакетов и shrink директории, если это возможно.
- Для тестов и бенчмарков используется детерминированный 64-bit hasher, чтобы избежать деградации из-за плохого распределения.

## Полезные команды для pprof

```bash
go tool pprof -top metrics/raw/profiles/insert_100000_cpu.pprof
go tool pprof -top metrics/raw/profiles/insert_100000_heap.pprof
```