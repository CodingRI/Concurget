package downloader

import (
	"io"
	"net/http"
	"os"
	"path"

	"net/url"

	//"errors"
	"fmt"
)

func Download(rawURL string) error {

		resp, err := http.Get(rawURL)
		
		if err != nil {
			return err
		}

		
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status: %d", resp.StatusCode);
		}

		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}

		fileName := path.Base(u.Path)

		//handling the "/" files
		if fileName == "." || fileName == "/" {
			fileName = "download.bin"
		}
		file, err := os.Create(fileName)
		if err != nil {
			return err
		}

		defer file.Close()

		bytesWritten, err := io.Copy(file, resp.Body)

		if err != nil {
			return err
		}

		fmt.Printf("%s downloaded (%d bytes)\n", fileName, bytesWritten)
		return nil
	
}
