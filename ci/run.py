#!/usr/bin/env python3
"""Entry point for running Dagger pipelines from Woodpecker CI."""

import argparse
import asyncio
import os
import sys

import anyio
import dagger

REGISTRY = "artifacts.toji.homes"
IMAGE_NAME = "gopedia-svc"


async def _build_and_push(client: dagger.Client, token: dagger.Secret, sha: str) -> str:
    tag = sha[:7]
    # Build from repo root — the entire repo is the build context.
    container = (
        client.host()
        .directory(".")
        .docker_build(dockerfile="Dockerfile")
        .with_registry_auth(REGISTRY, "woodpecker", token)
    )

    sha_addr = f"{REGISTRY}/neunexus/{IMAGE_NAME}:{tag}"
    latest_addr = f"{REGISTRY}/neunexus/{IMAGE_NAME}:latest"
    sha_ref, _ = await asyncio.gather(
        container.publish(sha_addr),
        container.publish(latest_addr),
    )
    return f"✓ {IMAGE_NAME}: {sha_ref}"


async def _validate(client: dagger.Client) -> str:
    await (
        client.host()
        .directory(".")
        .docker_build(dockerfile="Dockerfile")
    )
    return f"✓ {IMAGE_NAME}: build OK"


async def cmd_build(sha: str, token_val: str) -> None:
    runner_host = os.getenv("DAGGER_RUNNER_HOST", "")
    print(f"DAGGER_RUNNER_HOST={runner_host!r}")
    if runner_host:
        os.environ["DAGGER_RUNNER_HOST"] = runner_host
        os.environ["_EXPERIMENTAL_DAGGER_RUNNER_HOST"] = runner_host
    async with dagger.Connection() as client:
        token = client.set_secret("registry_token", token_val)
        result = await _build_and_push(client, token, sha)
    print(result)


async def cmd_validate() -> None:
    runner_host = os.getenv("DAGGER_RUNNER_HOST", "")
    print(f"DAGGER_RUNNER_HOST={runner_host!r}")
    if runner_host:
        os.environ["DAGGER_RUNNER_HOST"] = runner_host
        os.environ["_EXPERIMENTAL_DAGGER_RUNNER_HOST"] = runner_host
    async with dagger.Connection() as client:
        result = await _validate(client)
    print(result)


def main() -> None:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_build = sub.add_parser("build")
    p_build.add_argument("--sha", required=True)

    sub.add_parser("validate")

    args = parser.parse_args()

    if args.cmd == "build":
        token_val = os.environ.get("REGISTRY_TOKEN", "")
        if not token_val:
            print("ERROR: REGISTRY_TOKEN not set", file=sys.stderr)
            sys.exit(1)
        anyio.run(cmd_build, args.sha, token_val)

    elif args.cmd == "validate":
        anyio.run(cmd_validate)


if __name__ == "__main__":
    main()
