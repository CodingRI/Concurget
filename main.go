package main

import (
	"fmt"
	//"concurget/downloader"
	"concurget/internal"
)


func main() {
	fmt.Println("Reading file...")

	urls, err := internal.ReadURLs("urls.txt")

	if err != nil {
		fmt.Println(err)
		return
	}

	for  _, url := range urls {
			fmt.Println(url)
	
			// message := downloader.Download(url);
		
			// fmt.Println(message)
		}
	
}