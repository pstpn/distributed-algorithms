from __future__ import annotations

import argparse
from pathlib import Path


SIZES = [
    20000,
    50000,
    100000,
    250000,
    500000,
    750000,
    1100000,
    2000000,
    5000000,
    7500000,
]


def make_key(index: int, key_len: int) -> str:
    prefix = f"k{index:0{key_len - 1}d}"
    if len(prefix) >= key_len:
        return prefix[-key_len:]
    return prefix + "x" * (key_len - len(prefix))


def generate_one(path: Path, rows: int, key_len: int) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="") as f:
        f.write("text,y\n")
        for i in range(rows):
            key = make_key(i, key_len)
            val = (i % 1000) / 1000.0
            f.write(f"{key},{val:.6f}\n")


def main() -> int:
    parser = argparse.ArgumentParser(description="generate benchmark csv datasets")
    parser.add_argument("--out-dir", default="data/bench", help="output directory")
    parser.add_argument("--key-len", type=int, default=32, help="fixed key length")
    args = parser.parse_args()

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    manifest_path = out_dir / "manifest.txt"
    manifest_lines: list[str] = []

    for n in SIZES:
        file_name = f"train_data_bench_{n}.csv"
        file_path = out_dir / file_name
        print(f"generating {file_path} ({n} rows)")
        generate_one(file_path, n, args.key_len)
        manifest_lines.append(file_path.as_posix())

    with manifest_path.open("w", encoding="utf-8", newline="") as mf:
        for line in manifest_lines:
            mf.write(f"{line}\n")

    print(f"successfully generated {len(SIZES)} datasets in {out_dir}")
    print(f"manifest written to {manifest_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
