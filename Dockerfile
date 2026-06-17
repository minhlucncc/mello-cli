# Build stage.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 go build \
	-ldflags "-s -w \
	-X github.com/minhlucncc/mello-cli/cmd.Version=${VERSION} \
	-X github.com/minhlucncc/mello-cli/cmd.Commit=${COMMIT} \
	-X github.com/minhlucncc/mello-cli/cmd.Date=${DATE}" \
	-o /out/mello .

# Runtime stage: a minimal, non-root image.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/mello /usr/local/bin/mello
ENTRYPOINT ["/usr/local/bin/mello"]
