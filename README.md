# ConcurGet — A Concurrent File Downloader in Go

> Built to actually *feel* Go's concurrency model instead of reading about it.Every concept is explained standalone, then tied back to where it's used in ConcurGet, and (since you're coming from Node) mapped to the closest JS equivalent so the mental model isn't starting from zero.

---

## Table of Contents

1. [The Core Mental Shift (Node → Go)](#the-core-mental-shift-node--go)
2. [Project Flow](#project-flow)
3. [Folder Structure](#folder-structure)
4. [Goroutines](#1-goroutines)
5. [WaitGroup](#2-waitgroup)
6. [Channels](#3-channels)
7. [Worker Pool](#4-worker-pool)
8. [Context](#5-context)
9. [Select](#6-select)
10. [HTTP Client](#7-http-client)
11. [Streaming Downloads](#8-streaming-downloads)
12. [Readers and Writers](#9-readers-and-writers)
13. [Result Struct](#10-result-struct)
14. [CLI Flags](#11-cli-flags)
15. [Error Handling](#12-error-handling)
16. [defer](#13-defer)
17. [Retry Logic](#14-retry-logic)
18. [Graceful Shutdown](#15-graceful-shutdown)
19. [Cheat Sheet: Node → Go](#cheat-sheet-node--go)
20. [What I Learned](#what-i-learned)
21. [Future Improvements](#future-improvements)

---

## The Core Mental Shift (Node → Go)

Node's concurrency story is: **one thread, one event loop, async I/O via callbacks/promises.** You never think about threads because there basically aren't any (well, libuv's thread pool exists, but you don't touch it directly).

Go's concurrency story is: **many lightweight "goroutines" that the Go runtime schedules onto a small number of OS threads, and they talk to each other through channels instead of shared state.**

The biggest mindset flip:

| Node | Go |
|---|---|
| Single-threaded, non-blocking I/O | Multi-threaded under the hood, but you write blocking-looking code |
| `async/await`, Promises | Goroutines + channels |
| `Promise.all([...])` | `sync.WaitGroup` |
| `EventEmitter` / pub-sub | Channels |
| `AbortController` / `AbortSignal` | `context.Context` |
| `Promise.race([...])` | `select {}` |
| `try/catch` | `if err != nil {}` |
| `finally` | `defer` |
| npm event loop phases | Go scheduler (M:N threading) |

Keep this table in your head. Every section below basically expands one row of it.

---

## Project Flow

```
             urls.txt
                 │
                 ▼
        ReadURLs() function
                 │
                 ▼
          []string (slice)
                 │
                 ▼
          jobs channel
                 │
      ┌──────────┼──────────┐
      ▼          ▼          ▼
   Worker 1   Worker 2   Worker 3
      │          │          │
      └──────────┼──────────┘
                 ▼
         downloader.Download()
                 │
                 ▼
        HTTP Request + io.Copy
                 │
                 ▼
        Result returned
                 │
                 ▼
        results channel
                 │
                 ▼
          Main Goroutine
                 │
                 ▼
      Metrics + Final Summary
```

The interesting part: **workers never talk to each other directly.** They only communicate through channels — no shared array, no mutex, no "who's touching this variable right now" anxiety. If you've ever had a race condition bug in a Node worker_threads setup because two threads touched the same object, this is Go's answer to that: don't share the object, pass messages instead.

---

## Folder Structure

```
concurget/

cmd/
downloader/
internal/
logger/
metrics/
worker/

downloads/

main.go
urls.txt
```

Every folder is a Go **package** — think of a package the way you'd think of an npm module, except there's no `package.json`, no `node_modules`, and imports are resolved by folder path + module name (from `go.mod`) instead of a registry lookup at install time.

Each package has one job:

- `worker` — knows how to pull jobs and process them
- `downloader` — knows how to download a single file
- `metrics` — stores/aggregates stats
- `cmd` — parses CLI flags
- `internal` — Go's version of "private" — anything under `internal/` can only be imported by code inside this module, enforced by the compiler, not convention

That last point is worth sitting with: in Node, "internal" is just a naming convention and ESLint rule at best. In Go, `internal/` is a **compiler-enforced boundary**. You physically cannot `import` it from outside the module.

---

## Core Go Concepts

---

## 1. Goroutines

### The problem, Node-style first

In Node, if you wanted 5 downloads to happen "at once," you'd do something like:

```js
await Promise.all(urls.map(url => downloadFile(url)));
```

Under the hood, the event loop interleaves the I/O waits, so it *feels* concurrent even though it's one thread. That's concurrency via async I/O, not parallelism.

### Now in Go

Sequential execution — one task waits for the previous one:

```
Task A → Task B → Task C
```

If one download takes `3 seconds`, five sequential downloads take roughly `15 seconds`. Sound familiar? That's what you'd get in Node if you accidentally used a `for` loop with `await` inside instead of `Promise.all`.

Go's answer to "run these together":

```go
go Download(url)
```

The `go` keyword tells the Go runtime: *"run this function independently, don't block the caller."* This is roughly the semantic of firing off a Promise without awaiting it immediately — except goroutines are actual scheduled units of execution, not microtask-queue callbacks.

### Why goroutines are "lightweight"

```
Operating System
      ↓
  Few OS Threads
      ↓
Thousands of Goroutines
```

Node has **one** main thread doing your JS (plus libuv's thread pool for some I/O). Go's runtime multiplexes potentially thousands of goroutines onto a handful of OS threads — this is called **M:N scheduling** (M goroutines mapped onto N OS threads). A goroutine starts with a tiny (~2KB) stack that grows as needed, so spinning up 10,000 of them is genuinely cheap — nothing like spinning up 10,000 OS threads would be.

### How ConcurGet uses this

Instead of downloading files one after another, the project spins up multiple **workers**:

```go
go worker.Start(...)
```

Each worker runs independently. If Worker 1 is stuck on a slow file, Worker 2 and Worker 3 just keep going — no single slow request blocks the others, similar to how one slow `await fetch()` in Node doesn't block unrelated concurrent requests, except here it's real scheduled concurrency, not event-loop interleaving.

---

## 2. WaitGroup

### The problem

```go
go worker1()
go worker2()
go worker3()
```

`main()` reaches its end and the program just... exits. The workers never got a chance to finish. This is the Go equivalent of a Node script exiting before your fire-and-forget promises resolve because nothing was `await`-ed and the event loop had nothing left keeping it alive.

### The fix: `sync.WaitGroup`

Think of it as a counter Go tracks for you:

```
Counter = 0
```

Starting work:

```go
wg.Add(1)
```
```
Counter = 1
```

Three workers started → `Counter = 3`. Each worker finishes:

```go
wg.Done()
```

Counter ticks down: `3 → 2 → 1 → 0`. Then:

```go
wg.Wait()
```

blocks the calling goroutine until the counter hits zero.

**Node analogy:** this is almost exactly what `Promise.all([p1, p2, p3])` does for you — except `Promise.all` is built around a fixed array of Promises you already have, while `WaitGroup` is an explicit, mutable counter you increment/decrement yourself. It's closer to manually implementing a countdown latch than to `Promise.all`.

### How ConcurGet uses this

Every worker increments the WaitGroup before starting, and calls:

```go
defer wg.Done()
```

when it exits (more on `defer` later). The main goroutine calls `wg.Wait()` and blocks until every worker has genuinely finished — no downloads silently cut off mid-stream.

---

## 3. Channels

This is the single biggest "oh, THIS is why people say Go is different" feature.

### The mental model

Think of a conveyor belt in a factory:

```
Worker puts package → Conveyor Belt → Another worker receives package
```

Nobody reaches across and grabs something out of someone else's hands. They only interact through the belt.

**Node analogy:** the closest thing you already know is an `EventEmitter`, or better, a bounded async queue — messages get pushed and consumed, and the emitter/queue itself is the only shared object, not the underlying data.

### Creating and using one

```go
jobs := make(chan string)
```

Sending — like placing a package on the belt:

```go
jobs <- url
```

Receiving — like taking a package off the belt:

```go
url := <-jobs
```

### What does `<-` actually mean?

It's just an arrow showing **direction of data flow.**

Sending:
```go
jobs <- url     // url flows INTO jobs
```

Receiving:
```go
url := <-jobs   // value flows OUT of jobs, INTO url
```

### Channel direction types

```go
chan string     // can send AND receive
<-chan string   // receive-only — compiler blocks writes
chan<- Result   // send-only — compiler blocks reads
```

This is a genuinely nice feature with no real Node equivalent — you can declare a function parameter as "you may only read from this channel" and the **compiler** enforces it. In TypeScript the closest thing is a readonly type, but that's still just erased at runtime; Go's channel direction is enforced structurally at compile time, on an actual concurrency primitive.

### How ConcurGet uses this

Two channels drive the whole pipeline:

```
jobs → Workers → results
```

Workers receive from `jobs`, do the download, and send a `Result` into `results`. Nobody shares a slice or map directly between goroutines — which sidesteps an entire category of race-condition bugs you'd otherwise need a mutex (or careful `Promise` sequencing) to avoid.

---

## 4. Worker Pool

### The problem with going all-in on goroutines

Downloading 10,000 files naively:

```go
for _, url := range urls {
    go Download(url)
}
```

That's 10,000 goroutines fired at once. Even though goroutines are cheap, you'll still hit real limits:

- Too many simultaneous TCP connections
- Too many open file descriptors
- Memory usage climbing with every in-flight request
- The remote server rate-limiting or banning you

**Node analogy:** this is exactly why you don't do `await Promise.all(urls.map(fetchAndSave))` for 10,000 URLs — you'd reach for something like `p-limit` or a custom queue with a concurrency cap. A Go worker pool *is* that concurrency cap, just built from primitives instead of a library.

### The pattern

Instead of unbounded goroutines, spin up a **fixed number** of workers:

```
jobs → Worker 1
       Worker 2
       Worker 3
```

Each worker loops, effectively asking "is there another job?" — if yes, it processes it; if the channel's closed and empty, it exits. This is the **Worker Pool** pattern, and it's the single most reusable concurrency pattern in Go — you'll use it far beyond downloaders.

---

## 5. Context

### The mental model

Think of `context.Context` as a remote control that gets handed down through every layer of a call chain:

```
Context → Worker → HTTP Request → Database → Everything listening
```

The most common signal it carries is **cancel**. If the user hits `Ctrl+C`, the context gets cancelled, and *every* goroutine holding that context (however deep in the call stack) can react — without you manually tracking and stopping each one individually.

**Node analogy:** this maps closely to `AbortController` / `AbortSignal`. You create one controller, pass its `signal` down through `fetch()` calls and anything else that respects it, and calling `controller.abort()` propagates cancellation everywhere the signal was threaded through. Go's `context.Context` is the same idea, just baked into the standard library far more pervasively — almost every blocking Go API (HTTP, DB drivers, etc.) accepts a `ctx` as its first argument by convention.

### Context with timeout

```go
ctx, cancel := context.WithTimeout(parent, 5*time.Second)
```

This creates a context that auto-cancels after the given duration — equivalent to combining `AbortController` with a `setTimeout(() => controller.abort(), 5000)` in Node, except it's a single, idiomatic call in Go.

### How ConcurGet uses this

Every download function takes a context as its first parameter:

```go
Download(ctx, url)   // not Download(url)
```

This single change buys timeouts, manual cancellation, and graceful shutdown — all without touching the downloader's internal implementation.

---

## 6. Select

`select` is like `switch`, but built specifically for channel operations.

```go
select {
case <-ctx.Done():
    // context was cancelled — bail out
case job := <-jobs:
    // got a new job — process it
}
```

The worker waits on **multiple channel events at once**, and whichever fires first wins.

**Node analogy:** this is basically `Promise.race([ctx.done, nextJob])` — except it's a native language construct, not a library function wrapping an array of promises, and it can loop indefinitely inside a `for { select { ... } }` block to keep watching multiple channels forever. This exact pattern (`select` on a "done" channel plus a "work" channel) is *the* idiomatic way to make a long-running Go goroutine cancellable, and you'll see it constantly in real-world Go server code.

---

## 7. HTTP Client

Instead of the quick-and-dirty:

```go
http.Get(url)
```

ConcurGet builds the request explicitly:

```go
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
resp, err := client.Do(req)
```

**Node analogy:** `http.Get` is like calling bare `fetch(url)` with no options — works, but gives you no control. `NewRequestWithContext` + `client.Do` is like constructing a `fetch(url, { signal, headers, method })` — you get cancellation (via the context/signal), custom headers, timeouts, and retries as first-class options instead of an afterthought.

---

## 8. Streaming Downloads

### The beginner mistake

```
Internet → Entire file loaded into RAM → then written to Disk
```

For big files, this is a memory disaster — same mistake as doing `const buf = await response.arrayBuffer()` in Node for a multi-GB file instead of piping the stream.

### The Go way

```
Internet → Response Body → io.Copy() → Disk
```

Data flows piece by piece; memory usage stays roughly constant regardless of file size.

**Node analogy:** this is exactly `response.body.pipe(fs.createWriteStream(...))` (or the newer `stream.pipeline` / `Readable.pipe`). Go's `io.Copy(dst, src)` is the direct equivalent of Node's `.pipe()` — both move data through a fixed-size internal buffer instead of materializing the whole payload in memory.

---

## 9. Readers and Writers

Go's standard library is built almost entirely around two interfaces:

```go
type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }
```

A `Reader` *produces* data. A `Writer` *consumes* data. Examples:

- **Readers**: HTTP response body, an open file, a `strings.Reader`
- **Writers**: an open file, a network connection, an in-memory buffer

Because everything implements these two tiny interfaces, one function works everywhere:

```go
io.Copy(dst, src)
```

**Node analogy:** this is Go's version of the `stream.Readable` / `stream.Writable` abstraction — the reason `.pipe()` works identically whether the source is a file, an HTTP response, or a socket in Node is the same reason `io.Copy` works identically in Go: both ecosystems standardized on a minimal streaming interface that everything else implements. Go just takes it further — interfaces this small (one method!) are idiomatic everywhere, not just in the streams module.

---

## 10. Result Struct

Instead of a worker returning only an `error`, the downloader returns a `Result` struct:

```go
type Result struct {
    URL      string
    Filename string
    Bytes    int64
    Err      error
}
```

**Node analogy:** this is the Go version of resolving a Promise with a structured object like `{ url, filename, bytes, error }` instead of just throwing or returning a boolean. It keeps the worker "dumb" — it just packages up what happened and sends it down the `results` channel. The main goroutine is the only place that decides *how* to display or log it, which is a nice separation of concerns (workers don't know about `fmt.Println`, they just produce data).

---

## 11. CLI Flags

```
concurget -f urls.txt -c 5
```

- `-f` — input file path
- `-c` — number of workers

Implemented with Go's standard `flag` package.

**Node analogy:** this is the built-in equivalent of reaching for `commander` or `yargs` — except it ships in the standard library, so there's no `npm install` needed for basic flag parsing.

---

## 12. Error Handling

Go doesn't use exceptions for expected failures. Functions return a `(value, error)` pair:

```go
file, err := os.Open(path)
if err != nil {
    // handle it right here, right now
}
```

**Node analogy:** this replaces `try { ... } catch (err) { ... }`. The big philosophical difference: in Node, errors can bubble up silently through async call stacks until *someone* catches them (or crashes the process). In Go, every single call site that can fail forces you to explicitly check `err != nil` right there — more verbose, but it means you can't forget to handle a failure path the way you might forget a `.catch()` on a stray Promise.

---

## 13. defer

```go
file, err := os.Open(path)
if err != nil { return err }
defer file.Close()
```

`defer` tells Go: *"run this right before the current function returns — no matter which return path is taken, even if a panic happens."*

**Node analogy:** this is the closest thing Go has to `finally` — except `defer` is scoped to the *function*, is stackable (multiple `defer` calls run in LIFO order), and doesn't need a `try` block wrapped around anything. You just write `defer file.Close()` immediately after opening the file, and forget about it — cleanup is guaranteed regardless of how the function exits.

### How ConcurGet uses this

`defer wg.Done()` at the top of each worker's loop guarantees the WaitGroup counter decrements even if the worker hits an error mid-job and returns early.

---

## 14. Retry Logic

Network failures are frequently transient — a blip, a timeout, a momentary 503. Instead of giving up immediately, the downloader retries a failed request a few times before finally reporting failure.

**Node analogy:** the manual version of what a library like `p-retry` or `axios-retry` gives you — ConcurGet implements this by hand instead of pulling in a package, which is worth doing at least once to actually understand what those libraries are doing for you.

---

## 15. Graceful Shutdown

`Ctrl+C` doesn't kill the program instantly:

```
Ctrl+C
   ↓
Context cancelled
   ↓
Workers stop accepting new jobs
   ↓
In-flight downloads finish or cancel cleanly
   ↓
Summary printed
   ↓
Program exits
```

**Node analogy:** this is the Go equivalent of listening for `process.on('SIGINT', ...)`, calling `controller.abort()`, waiting for in-flight requests to wind down, then calling `process.exit()` — except in Go it's wired through `context.Context` + `select` + `WaitGroup` rather than an event listener callback.

---

## Cheat Sheet: Node → Go

| Concept | Node.js | Go |
|---|---|---|
| Run something concurrently | `Promise` / async function call | `go func() {...}()` |
| Wait for N concurrent tasks | `Promise.all([...])` | `sync.WaitGroup` |
| Pass messages between concurrent units | `EventEmitter` / queue | `chan T` |
| Bounded concurrency | `p-limit`, custom queue | Worker pool (`chan` + fixed goroutines) |
| Cancellation / timeout signal | `AbortController` / `AbortSignal` | `context.Context` |
| Wait on multiple async sources | `Promise.race([...])` | `select {}` |
| HTTP request w/ cancellation | `fetch(url, { signal })` | `http.NewRequestWithContext` |
| Streaming large payloads | `.pipe()` / `stream.pipeline` | `io.Copy(dst, src)` |
| Universal stream interface | `Readable` / `Writable` | `io.Reader` / `io.Writer` |
| Error handling | `try/catch` | `if err != nil` |
| Guaranteed cleanup | `finally` | `defer` |
| Retry wrapper | `p-retry` | Manual retry loop |
| Graceful shutdown | `process.on('SIGINT', ...)` | `context` cancellation + `select` |

---

## What I Learned

- How goroutines differ from OS threads (and why that makes them cheap)
- Why worker pools beat unlimited concurrency once you hit real-world limits
- How channels let goroutines coordinate *without* sharing memory directly
- Why `WaitGroup` exists — and what breaks without it
- How `context.Context` propagates cancellation top-to-bottom through a call chain
- How to make HTTP requests respect timeouts and cancellation
- Why `io.Copy()` keeps memory usage flat regardless of file size
- How Go's standard library leans on tiny, universal interfaces (`Reader`/`Writer`)
- Why Go prefers explicit `(value, error)` returns over exceptions
- How packages (and `internal/`) enforce project structure at the compiler level

---

## Future Improvements

- Per-file progress bars
- Resume interrupted downloads
- Download speed monitoring
- Configurable retry count
- Exponential backoff
- Structured logging
- Unit tests
- Integration tests
- Custom HTTP header support
- Authentication support

---

## Final Thoughts

This ended up being a much bigger lesson than "downloading files concurrently." It's a hands-on tour of Go's actual philosophy:

> **"Do not communicate by sharing memory; instead, share memory by communicating."**

Instead of locks scattered everywhere, goroutines pass messages through channels, the main goroutine coordinates everything, and `context` governs the lifetime of the whole application. If you keep coming back to this file, the fastest way to re-anchor is: **re-read the Node → Go cheat sheet first**, then jump to whichever numbered section you're fuzzy on.