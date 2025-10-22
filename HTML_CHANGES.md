# HTML Changes Not Showing in Browser?

If you're making HTML changes and they're not showing in the browser, here's how to fix it:

## Understanding the Issue

The problem is related to how you're running the server:

1. If you run with `gochin run start`, you're using a pre-compiled binary at `/usr/local/bin/gochin`
2. This binary doesn't automatically see your HTML changes because it's not reloading files

## Solution: I've implemented 3 fixes

### 1. Dynamic File Loading

- The server now automatically checks for file changes
- When you refresh the browser, it will detect modified HTML files
- No need to rebuild for HTML changes

### 2. You have THREE ways to run the server:

#### Option 1: Run directly from source (BEST during development)
```bash
# This will always use the latest HTML files
CGO_ENABLED=0 go run cmd/gochin/main.go run start
```

#### Option 2: Build locally (good for testing the binary)
```bash
make build
./gochin run start  # Using local binary
```

#### Option 3: Use the globally installed binary
```bash
# You'll still see HTML changes due to the live reload feature
gochin run start
```

### How the changes work:

1. I completely rewrote `file_loader.go` to use a caching system
2. It now checks the file modification time on each request
3. If the HTML file has changed, it automatically reloads it
4. The server logs when it loads a changed file

### Testing your changes:

1. Start the server (using any of the methods above)
2. Make changes to `internal/server/index.html`
3. Refresh your browser - changes should appear immediately!

## Troubleshooting

If you still don't see changes:
- Try clearing your browser cache (Ctrl+F5)
- Make sure you're editing the right file path
