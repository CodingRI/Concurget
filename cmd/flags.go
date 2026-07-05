package cmd

import (
	"flag"
)


type Config struct {
	File string
	Workers int
}

func ParseFlags() Config {
	file := flag.String("f", "urls.txt", "Input file")

	workers := flag.Int("c", 3, "Number of concurrent workers")

	flag.Parse()

	return Config{
		File : *file,
		Workers: *workers,
	}
}