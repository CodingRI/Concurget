package main

import (
	"context"
	"time"
	"fmt"
	"sync"

	

	"concurget/cmd"
	"concurget/downloader"
	"concurget/internal"
	"concurget/metrics"
	"concurget/worker"

)

func main() {

	config := cmd.ParseFlags()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	
	defer cancel()

	urls, err := internal.ReadURLs(config.File)
	if err != nil {
		fmt.Println(err)
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
		for _, url := range urls {
			jobs <- url
		}
		close(jobs)
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
			fmt.Println(result.Err)
			continue
		}

		m.Success++
		m.Bytes += result.Bytes

		fmt.Printf("Downloaded %s (%d bytes)\n",
			result.Filename,
			result.Bytes)
	}

	fmt.Println("----------- Summary -----------")
	fmt.Printf("Attempted : %d\n", m.Attempted)
	fmt.Printf("Success   : %d\n", m.Success)
	fmt.Printf("Failure   : %d\n", m.Failure)
	fmt.Printf("Bytes     : %d\n", m.Bytes)
}