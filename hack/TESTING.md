# Manual Testing Guide: live-updates

## Prerequisites

Build Glance and the mock upstream:

```bash
go build ./...
go build -o hack/mock-upstream/mock-upstream ./hack/mock-upstream/
```

## Starting the servers

In terminal 1 — start the mock upstream:

```bash
./hack/mock-upstream/mock-upstream -addr :9090
```

In terminal 2 — start Glance with the test config:

```bash
go run -race . --config hack/glance.live-test.yml
```

Open http://localhost:8080 in a browser.

## Viewing the raw SSE stream

```bash
curl -N http://localhost:8080/api/events
```

You should see `: ping` lines every 30 seconds when idle.

## Test scenarios

### 1. Basic update — bump triggers exactly one refresh

1. Open http://localhost:8080 in a browser. Note the counter value (should be 0).
2. Bump the counter:
   ```bash
   curl -X POST http://localhost:9090/bump
   ```
3. Within ~20 seconds, the "Counter (cache 10s)" widget should update without a page reload. The "Counter slow (cache 30m)" widget must **not** update.
4. The raw SSE stream should show exactly one `widget-updated` event for the 10s widget.

### 2. Coalescing — double bump produces one fetch

1. Bump the counter twice in quick succession:
   ```bash
   curl -X POST http://localhost:9090/bump && curl -X POST http://localhost:9090/bump
   ```
2. Within ~20 seconds, the 10s widget updates once. The mock upstream log should show the `/data` endpoint being hit once for that widget ID (not twice).

### 3. No spurious events — static widget

1. Wait for more than 30 seconds without bumping.
2. Verify the raw SSE stream shows only ping comments, no `widget-updated` events.
3. Verify the mock upstream log shows no `/data` requests for the slow (30m) widget.

### 4. Cache respected — slow widget not over-polled

1. Let the server run for 5 minutes.
2. Check the mock upstream log. The `/data` endpoint should be hit at most once every 30 minutes for the slow widget.

### 5. Two browser tabs simultaneously

1. Open two tabs on http://localhost:8080.
2. Bump the counter.
3. Both tabs should update independently within ~20 seconds.

### 6. Flaky service — monitor widget updates

1. The "Flaky Service" monitor widget alternates between OK and error every 10 seconds.
2. Verify that the monitor widget updates in the browser each time the status changes.

### 7. Mock upstream restart

1. Kill the mock upstream (Ctrl-C in terminal 1).
2. Wait for a 10-second tick. The monitor widget should show an error state.
3. Restart the mock upstream. Within ~20 seconds the widget should recover.

### 8. Glance restart with browser tab open

1. With a tab open on http://localhost:8080, restart Glance (Ctrl-C + rerun).
2. The browser's EventSource should automatically reconnect. You should not need to refresh the tab.
3. After reconnect, bumping the counter should still trigger widget updates.

### 9. Hot-reload — config file change

1. Touch the config file:
   ```bash
   touch hack/glance.live-test.yml
   ```
2. The server should reload. Check the terminal for "Config file changed, reloading...".
3. If pprof is enabled (`import _ "net/http/pprof"`), check for goroutine leaks:
   ```bash
   curl http://localhost:8080/debug/pprof/goroutine?debug=1
   ```
   The goroutine count should return to baseline after each reload.

### 10. Auth enforcement (if auth is configured)

Add auth to the test config:

```yaml
auth:
  secret-key: <base64-key>   # go run . --secret-key
  users:
    admin:
      password: testpass
```

Then verify:

```bash
curl -I http://localhost:8080/api/events     # expect 401
curl -I http://localhost:8080/api/widgets/1/content/  # expect 401
```

## Behind a reverse proxy

When Glance is behind a reverse proxy (nginx, Caddy, Traefik), SSE connections must remain open for the duration of the session. Common pitfalls:

| Symptom | Likely cause | Fix |
| ------- | ------------ | --- |
| Events stop after 60 s | Proxy read/idle timeout | Increase `proxy_read_timeout` (nginx) or equivalent |
| No events arrive at all | Proxy buffering enabled | Check that `X-Accel-Buffering: no` is forwarded (nginx strips it by default when not proxying upstream) |
| EventSource reconnects every few seconds | Proxy closes idle keep-alive connections | Increase proxy keep-alive or idle timeout |

To debug, open **DevTools → Network → EventStream** tab and watch for connection resets.
