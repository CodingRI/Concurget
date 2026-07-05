package downloader

type Result struct {
	URL   	   string
	Filename   string
	Bytes	   int64
	Err		   error
}