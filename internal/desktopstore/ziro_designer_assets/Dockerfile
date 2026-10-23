FROM --platform=$BUILDPLATFORM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e AS build
ARG ZIRO_REVISION=7b4597005e4801d4fb19661fcac6fc3380547cf2
ADD https://codeload.github.com/ZiroEDA/ziro-designer/tar.gz/${ZIRO_REVISION} /tmp/source.tar.gz
WORKDIR /source
RUN tar -xzf /tmp/source.tar.gz --strip-components=1 && rm /tmp/source.tar.gz
COPY . /adapter/
RUN node /adapter/adapt.mjs /source && cp -r /adapter aurago-adapter \
    && tar -czf /tmp/ziro-source.tar.gz . \
    && corepack enable && pnpm install --frozen-lockfile \
    && pnpm -C designer build \
    && cp /tmp/ziro-source.tar.gz designer/dist/source.tar.gz \
    && cp LICENSE NOTICE.md designer/dist/

FROM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e
LABEL org.opencontainers.image.source="https://github.com/antibyte/AuraGo" \
      org.opencontainers.image.licenses="GPL-3.0-or-later" \
      org.opencontainers.image.title="AuraGo Ziro Designer" \
      org.opencontainers.image.version="ziro-7b45970-1" \
      org.opencontainers.image.upstream.revision="7b4597005e4801d4fb19661fcac6fc3380547cf2"
WORKDIR /app
COPY --from=build /source/designer/dist/ /app/
COPY server.mjs /server.mjs
USER node
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s CMD node -e "fetch('http://127.0.0.1:8080/health').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"
CMD ["node", "/server.mjs"]
