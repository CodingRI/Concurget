package logger

import (
	"log"
	"os"
)

var Info = log.New(
	os.Stdout,
	"[INFO] ",
	log.LstdFlags,
)

var Error = log.New(
	os.Stderr,
	"[ERROR] ",
	log.LstdFlags,
)