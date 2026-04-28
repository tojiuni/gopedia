"""gopedia-ci: Dagger module for building and pushing the gopedia-svc image."""

import asyncio
from typing import Annotated

import dagger
from dagger import Arg, DefaultPath, Secret, function, object_type

REGISTRY = "artifacts.toji.homes"
IMAGE_NAME = "gopedia-svc"


@object_type
class GopediaCi:
    """CI pipelines for the gopedia service."""

    @function
    async def build(
        self,
        source: Annotated[dagger.Directory, DefaultPath("/")],
        token: Annotated[Secret, Arg(name="token")],
        sha: str,
    ) -> str:
        """Build and push gopedia-svc image to the registry.

        Usage:
          dagger call build --token=env:REGISTRY_TOKEN --sha=$(git rev-parse HEAD)
        """
        tag = sha[:7]
        container = (
            source
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

    @function
    async def validate(
        self,
        source: Annotated[dagger.Directory, DefaultPath("/")],
    ) -> str:
        """Build gopedia-svc image without pushing (for PR validation).

        Usage:
          dagger call validate
        """
        await source.docker_build(dockerfile="Dockerfile")
        return f"✓ {IMAGE_NAME}: build OK"
