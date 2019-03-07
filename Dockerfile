FROM alpine

COPY ./bin/dispatcher /dispatcher
ENTRYPOINT ["/dispatcher"]