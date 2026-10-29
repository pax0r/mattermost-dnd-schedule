FROM node:24-bookworm-slim AS node

FROM golang:1.26.7-bookworm

RUN apt-get update \
    && apt-get install --no-install-recommends -y ca-certificates gcc git make python3 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/npm
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx

# Match the host user so generated lockfiles and artifacts remain editable.
ARG LOCAL_UID=1000
ARG LOCAL_GID=1000
RUN groupadd --gid "${LOCAL_GID}" builder \
    && useradd --uid "${LOCAL_UID}" --gid "${LOCAL_GID}" --create-home builder \
    && mkdir -p /cache/go-build /cache/go-mod /cache/npm /workspace/webapp/node_modules \
    && chown -R builder:builder /cache /workspace

ENV GOCACHE=/cache/go-build \
    GOMODCACHE=/cache/go-mod \
    npm_config_cache=/cache/npm \
    GOTOOLCHAIN=local

WORKDIR /workspace
USER builder
ENTRYPOINT ["make"]
CMD ["verify"]
