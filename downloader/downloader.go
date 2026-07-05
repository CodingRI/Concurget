package downloader

import (
	"context"
	"io"
	"net/http"
	"os"
	"path"

	"net/url"

	//"errors"
	"fmt"
	"time"
)

func Download(ctx context.Context, rawURL string) Result {
	client := &http.Client{}

	const maxRetries = 3
	
	var (
		resp *http.Response
		err  error
	)
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
	
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			rawURL,
			nil,
		)
		if err != nil {
			return Result{
				URL: rawURL,
				Err: err,
			}
		}
	
		resp, err = client.Do(req)
	
		// Success
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
	
		// Close body before retrying
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	
		// Don't sleep after the last attempt
		if attempt < maxRetries {
			fmt.Printf("Retry %d/%d: %s\n", attempt, maxRetries, rawURL)
			time.Sleep(time.Second)
		}
	}
	
	// Final failure
	if err != nil {
		return Result{
			URL: rawURL,
			Err: err,
		}
	}
	
	if resp.StatusCode != http.StatusOK {
		return Result{
			URL: rawURL,
			Err: fmt.Errorf("unexpected status %d", resp.StatusCode),
		}
	}
	
	defer resp.Body.Close()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{
			URL: rawURL,
			Err: fmt.Errorf("unexpected status: %d", resp.StatusCode),
		}
	}

		u, err := url.Parse(rawURL)
		if err != nil {
			return Result {
				URL : rawURL,
				Err: err,
			}
		}

		fileName := path.Base(u.Path)

		//handling the "/" files
		if fileName == "." || fileName == "/" {
			fileName = "download.bin"
		}
		

		os.MkdirAll("Downloads", 0755)

		filePath := path.Join("Downloads", fileName)

		file, err := os.Create(filePath)

		if err != nil {
			return Result {
				URL : rawURL,
				Err: err,
			}
		}

		defer file.Close()

		bytesWritten, err := io.Copy(file, resp.Body)

		if err != nil {
			return Result {
				URL : rawURL,
				Err: err,
			}
		}

		fmt.Printf("%s downloaded (%d bytes)\n", fileName, bytesWritten)
		
		
		return Result{
			URL:      rawURL,
			Filename: fileName,
			Bytes:    bytesWritten,
			Err:      nil,
		}
	
}
