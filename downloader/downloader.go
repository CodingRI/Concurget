package downloader

import (
	"net/http"
)

func Download(url string) error {

		resp, err := http.Get(url)
		if err != nil {
			return err
		}

		defer resp.Body.Close()

		return nil
	
}