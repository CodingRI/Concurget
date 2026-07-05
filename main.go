package main

import (
	"context"
	"time"
	"fmt"
	"sync"
	"os"
	"os/signal"
	"syscall"
	

	

	"concurget/cmd"
	"concurget/downloader"
	"concurget/internal"
	"concurget/metrics"
	"concurget/worker"
	"concurget/logger"

)

func main() {

	config := cmd.ParseFlags()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	
	defer cancel()

	signalChan := make(chan os.Signal, 1)

	signal.Notify(
		signalChan,
		os.Interrupt,
		syscall.SIGTERM,
	)

	go func() {
		<-signalChan

		fmt.Println("\nReceived interrupt signal")
		cancel()
	}()

	urls, err := internal.ReadURLs(config.File)
	if err != nil {
		logger.Error.Println(err)
		return
	}

	jobs := make(chan string)
	results := make(chan downloader.Result)

	var wg sync.WaitGroup

	for i := 1; i <= config.Workers; i++ {

		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			worker.Start(ctx, id, jobs, results)
		}(i)
	}

	// Producer
	go func() {
		defer close(jobs)
	
		for _, url := range urls {
			select {
			case <-ctx.Done():
				return
			case jobs <- url:
			}
		}
	}()

	// Close results after all workers finish
	go func() {
		wg.Wait()
		close(results)
	}()

	m := metrics.Metrics{}

	for result := range results {

		m.Attempted++

		if result.Err != nil {
			m.Failure++
			logger.Error.Println(result.Err)
			continue
		}

		m.Success++
		m.Bytes += result.Bytes

		logger.Info.Printf("Downloaded %s (%d bytes)\n",
			result.Filename,
			result.Bytes)
	}

	fmt.Println("----------- Summary -----------")
	fmt.Printf("Attempted : %d\n", m.Attempted)
	fmt.Printf("Success   : %d\n", m.Success)
	fmt.Printf("Failure   : %d\n", m.Failure)
	fmt.Printf("Bytes     : %d\n", m.Bytes)
}