# Vicary

Vicary is a Docker image pull through cache for dockerhub and other public docker registries
implemented solely using nginx.

It is an alternative to Docker's [pull through cache][pull-through].

[pull-through]: https://docs.docker.com/registry/recipes/mirror/

# Demo

Here is an example of how to run Vicary to test it out:
```
docker run -p 80:80 -e VICARY_PORT=80  -e VICARY_SCHEME=http -e VICARY_STORE=/tmp -e VICARY_RESOLVER=1.1.1.1 rvba/vicary:latest
```

The example usage below assume you've run the command above.

## Transparent docker hub cache

Vicary can be used as a transparent cache for dockerhub images using docker's `registry-mirrors` feature.

Configure docker to use the Vicary instance as a registry mirror and an "insecure registry"
since we're using http here for simplicity:
```
{
  [...]
  "registry-mirrors": [
    "http://localhost"
  ],
  "insecure-registries": [
    "localhost"
  ],
  [...]
```

Usage example:
```
# Initial docker pull will populate the cache
docker pull python:3.10

# Remove the image
docker image rm python:3.10

# Subsequent pulls will hit the Vicary cache without the Vicary having to re-download the image
# from dockerhub.
docker pull python:3.10
```

## Caching proxy

If you'd rather not change the docker configuration you can also use the pull through cache directly.
In this case you can pull (and cache) images not only from dockerhub but also quay.io and gcr.io.

```
# Pulling an image from dockerhub
docker pull localhost/docker.io/library/python:3.10

# Pulling an image from quay.io 
docker pull localhost/quay.io/jitesoft/debian:10

# Pulling an image from gcr.io
docker pull localhost/gcr.io/google-containers/busybox:1.27
```

# Production deployment

If you want to use Vicary in production, you probably want to configure it differently compared
to the example given above:

- VICARY_SCHEME should be set to https and Vicary must be run behind a TLS-terminating frontend.
- VICARY_DOCKER_IO_B64_AUTH must be set to a base64-encoded docker token to get Vicary to perform
  authenticated calls to docker.io and thus not hit docker's [download rate limit][docker-rate-limit].

[docker-rate-limit]: https://docs.docker.com/docker-hub/download-rate-limit/
