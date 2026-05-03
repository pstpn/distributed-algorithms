from __future__ import annotations

import argparse
import random
from pathlib import Path


SIZES = [
    5000,
    10000,
    20000,
    35000,
    50000,
    75000,
    100000,
    250000,
    500000,
    750000,
    1000000,
]


def generate_one(
    path: Path,
    rows: int,
    min_x: float,
    max_x: float,
    min_y: float,
    max_y: float,
    rng: random.Random,
) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="") as f:
        for i in range(rows):
            _ = i
            x = rng.uniform(min_x, max_x)
            y = rng.uniform(min_y, max_y)
            f.write(f"{x:.6f},{y:.6f}\n")


def main() -> int:
    parser = argparse.ArgumentParser(description="generate benchmark csv datasets")
    parser.add_argument("--out-dir", default="data/bench", help="output directory")
    parser.add_argument("--min-x", type=float, default=-180.0, help="minimum x")
    parser.add_argument("--max-x", type=float, default=180.0, help="maximum x")
    parser.add_argument("--min-y", type=float, default=-90.0, help="minimum y")
    parser.add_argument("--max-y", type=float, default=90.0, help="maximum y")
    parser.add_argument("--seed", type=int, default=None, help="random seed for reproducible output")
    args = parser.parse_args()

    if args.min_x >= args.max_x or args.min_y >= args.max_y:
        parser.error("invalid range: expected min < max for both axes")

    rng = random.Random(args.seed)

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    manifest_path = out_dir / "manifest.txt"
    manifest_lines: list[str] = []

    for n in SIZES:
        file_name = f"train_data_bench_{n}.csv"
        file_path = out_dir / file_name
        print(f"generating {file_path} ({n} rows)")
        generate_one(file_path, n, args.min_x, args.max_x, args.min_y, args.max_y, rng)
        manifest_lines.append(file_path.as_posix())

    with manifest_path.open("w", encoding="utf-8", newline="") as mf:
        for line in manifest_lines:
            mf.write(f"{line}\n")

    print(f"successfully generated {len(SIZES)} datasets in {out_dir}")
    print(f"manifest written to {manifest_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
