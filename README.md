# ConcurGet — A Concurrent File Downloader in Go

> A beginner-friendly Go project built to learn concurrency, worker pools, channels, HTTP clients, file streaming, context cancellation and idiomatic Go programming.

---


# Project Flow

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

The interesting part is that **workers never communicate with each other**.

They only communicate through channels.

---

# Folder Structure

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

Every folder is a Go package.

Each package has one responsibility.

For example,

* `worker` knows how to process jobs.
* `downloader` knows how to download files.
* `metrics` stores statistics.
* `cmd` parses command-line flags.

This keeps the code modular and easier to maintain.

---

# Core Go Concepts

---

# 1. Goroutines

The very first important feature of Go.

Most languages execute code like this.

```
Task A

↓

Task B

↓

Task C
```

Each task waits for the previous one to finish.

This is called **sequential execution**.

Suppose downloading one file takes

```
3 seconds
```

Downloading five files sequentially takes approximately

```
15 seconds
```

Now imagine this.

```
Task A
Task B
Task C
Task D
Task E
```

all start together.

That is what a Goroutine does.

Creating one is surprisingly simple.

```go
go Download(url)
```

The keyword `go` tells the Go runtime

> "Run this function independently."

Notice something.

A Goroutine is **not an operating system thread.**

Instead

```
Operating System

↓

Few Threads

↓

Thousands of Goroutines
```

The Go runtime schedules Goroutines onto a smaller number of threads.

This is why Goroutines are lightweight.

---

### How this project uses Goroutines

Instead of downloading one file after another,

the project creates multiple workers.

```go
go worker.Start(...)
```

Each worker runs independently.

If one worker is downloading a slow file,

another worker can continue downloading different files.

---

# 2. WaitGroup

Problem.

Suppose we start three Goroutines.

```go
go worker1()
go worker2()
go worker3()
```

The `main()` function reaches the end.

What happens?

The program exits immediately.

The workers don't get enough time to finish.

We need a mechanism to wait.

That is exactly what `sync.WaitGroup` does.

Think of it like a counter.

Initially

```
Counter = 0
```

Starting work

```go
wg.Add(1)
```

```
Counter = 1
```

Three workers

```
Counter = 3
```

Each worker finishes

```go
wg.Done()
```

Counter decreases.

```
3

↓

2

↓

1

↓

0
```

Finally

```go
wg.Wait()
```

blocks until the counter reaches zero.

---

### How this project uses WaitGroup

Every worker increments the WaitGroup.

When a worker finishes processing all jobs,

it calls

```go
defer wg.Done()
```

The main Goroutine waits until all workers exit.

---

# 3. Channels

Channels are probably the biggest "Go" feature.

Think of a conveyor belt inside a factory.

```
Worker puts package

↓

Conveyor Belt

↓

Another worker receives package
```

Nobody touches each other.

They communicate only through the conveyor belt.

Channels work exactly like that.

Creating one

```go
jobs := make(chan string)
```

This creates a channel capable of transporting strings.

Sending

```go
jobs <- url
```

Imagine placing a package onto the conveyor belt.

Receiving

```go
url := <-jobs
```

Imagine taking a package from the conveyor belt.

---

## What does `<-` mean?

This symbol confuses almost every beginner.

It simply represents the **direction of data flow**.

### Sending

```go
jobs <- url
```

means

```
url

↓

jobs channel
```

### Receiving

```go
url := <-jobs
```

means

```
jobs channel

↓

url variable
```

The arrow literally points in the direction data is moving.

---

### Channel Types

You will also see

```go
chan string
```

meaning

"Can send and receive."

Sometimes you'll see

```go
<-chan string
```

This means

"Receive-only."

The compiler prevents writing to it.

Similarly,

```go
chan<- Result
```

means

"Send-only."

The compiler prevents reading from it.

This is a fantastic feature because it catches mistakes at compile time.

---

### How this project uses Channels

The project has two channels.

```
jobs

↓

Workers

↓

results
```

Workers receive jobs.

Workers send results.

Nobody shares variables directly.

---

# 4. Worker Pool

Imagine downloading 10,000 files.

Naively

```go
go Download(...)
```

10,000 times.

That creates 10,000 Goroutines.

Possible problems

* Too many TCP connections.
* Too many open files.
* Excessive memory usage.
* Server rate limits.

Instead we create only

```
3 workers
```

```
jobs

↓

Worker 1

Worker 2

Worker 3
```

Workers continuously ask

> "Do you have another job?"

If yes,

they process it.

Otherwise

they exit.

This is called a **Worker Pool**.

---

# 5. Context

Context is one of the most important packages in Go.

Think of Context as a remote control.

```
Context

↓

Worker

↓

HTTP Request

↓

Database

↓

Everything listening
```

The Context carries signals.

The most common signal is

```
Cancel
```

Suppose the user presses

```
Ctrl+C
```

The Context gets cancelled.

Every Goroutine using that Context receives the cancellation.

Nobody needs to manually stop every worker.

---

### Context With Timeout

```go
ctx, cancel := context.WithTimeout(...)
```

creates a Context that automatically expires after a given duration.

If a download hangs forever,

the HTTP request stops automatically.

---

### How this project uses Context

Every download receives

```go
Download(ctx, url)
```

instead of

```go
Download(url)
```

This allows

* timeouts
* cancellation
* graceful shutdown

without changing the downloader implementation.

---

# 6. Select

`select` is like `switch`, but for channels.

Example

```go
select {

case <-ctx.Done():

case job := <-jobs:

}
```

The worker waits for two events.

Either

```
A new job arrives
```

or

```
The Context is cancelled.
```

Whichever happens first gets executed.

This is heavily used in Go servers.

---

# 7. HTTP Client

Instead of

```go
http.Get(...)
```

the project creates

```go
req, _ := http.NewRequestWithContext(...)
```

and executes it using

```go
client.Do(req)
```

Why?

Because it gives much more control.

Now requests respect cancellation and timeouts.

---

# 8. Streaming Downloads

One beginner misconception is

```
Internet

↓

Entire file in RAM

↓

Disk
```

That would be terrible.

Instead Go streams data.

```
Internet

↓

Response Body

↓

io.Copy()

↓

Disk
```

The file is copied piece by piece.

Memory usage stays almost constant.

Even huge files can be downloaded efficiently.

---

# 9. Readers and Writers

Go's standard library revolves around two interfaces.

```
Reader

Writer
```

A Reader can produce data.

A Writer can consume data.

Examples

Reader

* HTTP Response
* File
* String

Writer

* File
* Network Connection
* Memory Buffer

Because everything follows these interfaces,

the same function

```go
io.Copy(dst, src)
```

works everywhere.

That is one of the most elegant parts of Go.

---

# 10. Result Struct

Instead of returning only

```go
error
```

the downloader returns

```go
Result
```

containing

* URL
* Filename
* Bytes
* Error

This keeps the worker simple.

Workers only send Results.

The main Goroutine decides how to display them.

---

# 11. CLI Flags

The project supports

```
-f
```

Input file.

and

```
-c
```

Number of workers.

Internally Go provides the

```go
flag
```

package for this.

Example

```
concurget -f urls.txt -c 5
```

---

# 12. Error Handling

Unlike many languages,

Go does not use exceptions for normal failures.

Instead,

functions return

```go
(value, error)
```

Example

```go
file, err := os.Open(...)
```

If

```go
err != nil
```

the function failed.

This style makes failures explicit and easy to follow.

---

# 13. defer

Imagine

```
Open File

↓

Read

↓

Process

↓

Return
```

If we forget to close the file,

resources leak.

Instead

```go
defer file.Close()
```

tells Go

> "Execute this just before the current function exits."

This guarantees cleanup even if an error occurs later.

---

# 14. Retry Logic

Network failures are often temporary.

Instead of failing immediately,

the downloader retries failed requests several times before giving up.

This makes the downloader more reliable against transient server or network issues.

---

# 15. Graceful Shutdown

Pressing Ctrl+C does not immediately terminate the program.

Instead

```
Ctrl+C

↓

Context cancelled

↓

Workers stop accepting new jobs

↓

Current downloads finish or cancel

↓

Summary printed

↓

Program exits
```

This is exactly how many production command-line tools behave.

---

# What I Learned

By building this project I learned

* How Goroutines differ from threads.
* Why Worker Pools are better than unlimited concurrency.
* How Channels safely transfer work.
* Why WaitGroups are necessary.
* How Context propagates cancellation.
* How HTTP requests can respect timeouts.
* Why `io.Copy()` streams data efficiently.
* How Go's standard library relies heavily on interfaces.
* Why Go prefers explicit error handling instead of exceptions.
* How packages help organize large projects.

---

# Future Improvements

* Progress bars for each download.
* Resume interrupted downloads.
* Download speed monitoring.
* Configurable retry count.
* Exponential backoff.
* Better structured logging.
* Unit tests.
* Integration tests.
* Support for HTTP headers.
* Authentication support.

---

# Final Thoughts

This project became much more than a file downloader.

It served as a practical introduction to Go's philosophy:

> **"Do not communicate by sharing memory; instead, share memory by communicating."**

Instead of relying on locks everywhere, Goroutines communicate through Channels, the main Goroutine coordinates work, and Context manages the lifetime of the entire application.

