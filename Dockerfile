FROM nginx:1.21.4

ARG VICARY_RESOLVER=127.0.0.1
ENV VICARY_RESOLVER=$VICARY_RESOLVER

ARG VICARY_HEALTH_RESPONSE_BODY=OK
ENV VICARY_HEALTH_RESPONSE_BODY=$VICARY_HEALTH_RESPONSE_BODY

ARG VICARY_CACHE_FREE_SIZE=20m
ENV VICARY_CACHE_FREE_SIZE=$VICARY_CACHE_FREE_SIZE

ARG VICARY_CACHE_INACTIVE=50y
ENV VICARY_CACHE_INACTIVE=$VICARY_CACHE_INACTIVE

ARG VICARY_DOCKER_IO_B64_AUTH=
ENV VICARY_DOCKER_IO_B64_AUTH=$VICARY_DOCKER_IO_B64_AUTH

COPY default.conf.template /etc/nginx/templates/
COPY proxy.conf /etc/nginx/
