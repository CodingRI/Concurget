package worker

import (
	"fmt"
	"concurget/downloader"
	"context"
)


func Start(ctx context.Context, id int, jobs <-chan string, results chan<- downloader.Result) {
	for {

		select {
	
		case <-ctx.Done():
			fmt.Printf("[Worker %d] stopping...\n", id)
			return
	
		case url, ok := <-jobs:
	
			if !ok {
				return
			}
	
			fmt.Printf("[Worker %d] downloading %s\n", id, url)
	
			result := downloader.Download(ctx, url)
	
			results <- result
		}
	}
}