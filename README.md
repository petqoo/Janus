
````markdown
# Janus

> A blazingly fast live-reloader for Go applications with a built-in, crash-proof safety net and an interactive TUI dashboard.

Janus watches your Go project for file changes, automatically rebuilds your application, and intelligently handles runtime crashes. If a new version crashes on startup, Janus instantly reverts to the last stable version, ensuring your development flow is never interrupted.

---

## ✨ Core Features

- **Crash-Proof Safety Net**: Automatically reverts to the last stable version if a reload causes a runtime crash.
- **Interactive TUI Dashboard**: See your app's status, view its logs, and monitor Janus events in a clean, full-screen terminal interface.
- **Blazing Fast**: Built in Go for maximum performance.
- **Robust Process Management**: Reliably stops your application and any child processes it may have started, preventing "address already in use" errors.
- **Intelligent Reloading**: Uses debouncing to prevent a storm of rebuilds when saving multiple files.
- **Simple Configuration**: Configure everything in a single, clear `.janus.toml` file.

---

## 🚀 Installation

Make sure you have Go installed (version 1.18+ recommended). Then, run:

```bash
go install github.com/pitko/Janus/cmd/janus@latest
````

This will compile and install the `janus` binary in your Go bin directory.

---

##  Getting Started

Navigate to your Go project's root directory:

```bash
cd /path/to/your/project
```

Create a default configuration file:

```bash
janus init
```

This will create a `.janus.toml` file in your directory.

Run Janus!

```bash
janus
```

That's it! Janus will now display its TUI, watch your files, and keep your application running safely.

### Keyboard Shortcuts

* **Arrow Keys (↑/↓)**: Scroll the active panel.
* **Tab**: Switch between the Application and Aegis log panels.
* **q** or **Ctrl+C**: Quit Janus.

---

## ⚙ Configuration

The `janus init` command creates this default `.janus.toml` file. Customize it to fit your project's needs.

```toml
# .janus.toml

# Command to build your Go application.
[build]
  # This command is run from the `watch.root` directory.
  cmd = "go build -o ./tmp/app ./main.go"

  # The path to the binary that the command above creates.
  # This path is relative to the `watch.root` directory.
  bin = "./tmp/app"

# Settings for watching files.
[watch]
  # The application directory you want to watch.
  root = "."

  # Directories to ignore.
  exclude_dir = ["tmp", "vendor", ".git"]

# The crash-proof safety net configuration.
[safety_net]
  # How long to wait in milliseconds to see if the new app crashes.
  # If the app runs for this long, it's considered "stable".
  probation_ms = 2500
```

---

## 🛡️ How The Safety Net Works

1. **Detect**: Janus detects a file change in your project.
2. **Build**: It runs your build command to create a new *challenger* binary.
3. **Probation**: It stops the old stable app and starts the new *challenger* with a short probation timer (e.g., 2.5 seconds).
4. **Analyze**:

   *  **Success**: If the challenger survives the probation period, it's promoted to the new *stable* version.
   *  **Failure**: If the challenger crashes during probation, Janus logs the error, discards the failed binary, and immediately restarts the old stable version.

---

##  License

This project is licensed under the MIT License.

```

---

✅ You can now copy and paste this entire block directly into a `README.md` file in your project root.

Would you like me to add:
- Contribution guidelines?
- Build status or Go Report Card badges?
- A GIF of the TUI in action?

Just let me know!
```
