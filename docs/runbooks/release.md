# LazySS Release Runbook

1. Run local gates:

   ```sh
   gofmt -l .
   go vet ./...
   go test -race ./...
   go build ./cmd/lazyss
   ```

2. Tag:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. Confirm GitHub release artifacts and checksums.
