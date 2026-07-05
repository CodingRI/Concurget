package worker

import (
	"fmt"
	"concurget/downloader"
	"context"
)


func Start(ctx context.Context, id int, jobs <-chan string, results chan<- downloader.Result) {
	for url := range jobs {
		fmt.Printf("[Worker %d] downloading %s\n", id, url)

		result := downloader.Download(ctx, url)

		results <- result
	}
}