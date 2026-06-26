package internal

import (
	"bufio"
	"os"
)


func ReadURLs(filename string) ([]string, error){
	file, err := os.Open(filename)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	var urls []string

	for scanner.Scan() {
		line := scanner.Text()
		urls = append(urls, line)
	}

	if err := scanner.Err(); err != nil {
		return urls, err
	} 

	return urls,nil
}