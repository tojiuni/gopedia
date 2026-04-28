"""Dagger CI module for gopedia-svc (Go API + Python embedding service)."""

import asyncio
from typing import Annotated

import anyio
import dagger
from dagger import DefaultPath, Secret, dag, function, object_type
from dagger.mod import Arg

REGISTRY = "artifacts.toji.homes"
IMAGE = f"{REGISTRY}/neunexus/gopedia-svc"


@object_type
class GopediaCi:
    @function
    async def build(
        self,
        source: Annotated[dagger.Directory, DefaultPath("/")],
        token: Annotated[Secret, Arg(name="token")],
        sha: str,
    ) -> str:
        """Build, tag (:sha7 + :latest) and push the gopedia-svc image."""
        tag = sha[:7]
        sha_addr = f"{IMAGE}:{tag}"
        latest_addr = f"{IMAGE}:latest"

        container = (
            source.docker_build(dockerfile="Dockerfile")
            .with_registry_auth(REGISTRY, "woodpecker", token)
        )

        sha_ref, _ = await asyncio.gather(
            container.publish(sha_addr),
            container.publish(latest_addr),
        )
        return f"✓ gopedia-svc: {sha_ref}"

    @function
    async def validate(
        self,
        source: Annotated[dagger.Directory, DefaultPath("/")],
    ) -> str:
        """Build image without pushing (PR validation)."""
        await source.docker_build(dockerfile="Dockerfile")
        return "✓ gopedia-svc: build OK"
