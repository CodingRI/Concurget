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
)

func Download(ctx context.Context, rawURL string) Result {

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

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return Result{
			URL: rawURL,
			Err: err,
		}
	}

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
