#!/usr/bin/env python3
"""Check legacy HTML -> markdown migration coverage for city topic pages."""

from __future__ import annotations

import pathlib
import sys


REPO_ROOT = pathlib.Path(__file__).resolve().parents[1]


def topic_slugs_from_legacy(city: str) -> set[str]:
    city_dir = REPO_ROOT / city
    slugs: set[str] = set()
    for html in city_dir.glob("*.html"):
        name = html.name
        if name in {"index.html", "index-es.html"}:
            continue
        if name.endswith("-es.html"):
            continue
        slugs.add(name.removesuffix(".html"))
    return slugs


def topic_slugs_from_markdown(city: str) -> set[str]:
    content_dir = REPO_ROOT / "content" / city
    if not content_dir.exists():
        return set()
    slugs: set[str] = set()
    for md in content_dir.glob("*.en.md"):
        slugs.add(md.name.removesuffix(".en.md"))
    return slugs


def report_city(city: str) -> int:
    legacy = topic_slugs_from_legacy(city)
    markdown = topic_slugs_from_markdown(city)

    missing = sorted(legacy - markdown)
    extra = sorted(markdown - legacy)

    print(f"\n[{city}]")
    print(f"  legacy topics:   {len(legacy)}")
    print(f"  markdown topics: {len(markdown)}")

    if missing:
        print("  missing markdown migrations:")
        for slug in missing:
            print(f"    - {slug}")
    else:
        print("  missing markdown migrations: none")

    if extra:
        print("  markdown-only topics (no matching legacy html):")
        for slug in extra:
            print(f"    - {slug}")
    else:
        print("  markdown-only topics: none")

    return len(missing)


def main() -> int:
    total_missing = 0
    for city in ("boston", "seattle"):
        total_missing += report_city(city)

    print("\nSummary:")
    if total_missing == 0:
        print("  all legacy topics have markdown playbooks")
        return 0
    print(f"  {total_missing} legacy topic(s) still need markdown migration")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
