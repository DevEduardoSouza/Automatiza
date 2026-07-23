ARG POSTGRES_BASE=postgres:16-alpine
FROM ${POSTGRES_BASE}

ARG PGBACKREST_VERSION=2.58.0-r0

RUN apk add --no-cache "pgbackrest=${PGBACKREST_VERSION}"

COPY entrypoint-backup.sh /usr/local/bin/entrypoint-backup.sh

ENTRYPOINT ["/usr/local/bin/entrypoint-backup.sh"]
CMD ["postgres"]
