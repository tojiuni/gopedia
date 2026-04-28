#!/usr/bin/env python3
"""CLI wrapper for Dagger pipelines — called by Woodpecker steps."""

import argparse
import asyncio
import os
import sys

import anyio
import dagger

REGISTRY = "artifacts.toji.homes"
IMAGE = f"{REGISTRY}/neunexus/gopedia-svc"


async def _build(sha: str) -> None:
    token_val = os.environ["REGISTRY_TOKEN"]
    async with dagger.Connection(dagger.Config(log_output=sys.stderr)) as client:
        source = client.host().directory(".", exclude=[".venv", "__pycache__", ".git"])
        token = client.set_secret("registry_token", token_val)
        tag = sha[:7]
        container = (
            source.docker_build(dockerfile="Dockerfile")
            .with_registry_auth(REGISTRY, "woodpecker", token)
        )
        sha_ref, _ = await asyncio.gather(
            container.publish(f"{IMAGE}:{tag}"),
            container.publish(f"{IMAGE}:latest"),
        )
        print(f"✓ gopedia-svc: {sha_ref}")


async def _validate() -> None:
    async with dagger.Connection(dagger.Config(log_output=sys.stderr)) as client:
        source = client.host().directory(".", exclude=[".venv", "__pycache__", ".git"])
        await source.docker_build(dockerfile="Dockerfile")
        print("✓ gopedia-svc: build OK")


def main() -> None:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd")

    b = sub.add_parser("build")
    b.add_argument("--sha", required=True)

    sub.add_parser("validate")

    args = parser.parse_args()
    if args.cmd == "build":
        anyio.run(_build, args.sha)
    elif args.cmd == "validate":
        anyio.run(_validate)
    else:
        parser.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()
