"""CLI entry point for code_parser.

Usage:
    python -m flows.code_parser.cli parse --lang python < source.py
    python -m flows.code_parser.cli parse --lang go < source.go
"""
import sys
import json
import argparse

from flows.code_parser.parser import parse_source


def cmd_parse(args):
    source = sys.stdin.read()
    result = parse_source(source, args.lang)
    json.dump(result, sys.stdout, ensure_ascii=False)
    sys.stdout.write("\n")


def main():
    ap = argparse.ArgumentParser(description="Tree-sitter code parser CLI")
    sub = ap.add_subparsers(dest="command")

    parse_cmd = sub.add_parser("parse", help="Parse source code to JSON")
    parse_cmd.add_argument("--lang", required=True,
                           choices=["python", "go", "typescript"],
                           help="Source language")
    parse_cmd.set_defaults(func=cmd_parse)

    args = ap.parse_args()
    if not hasattr(args, "func"):
        ap.print_help()
        sys.exit(1)
    args.func(args)


if __name__ == "__main__":
    main()
