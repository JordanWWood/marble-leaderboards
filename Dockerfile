FROM golang:alpine
COPY LeaderboardsBackend .
CMD [ "./LeaderboardsBackend" ]