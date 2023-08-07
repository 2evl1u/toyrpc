package log

import (
	"log"
	"os"
)

var CommonLogger *log.Logger

var ErrorLogger *log.Logger

func init() {
	CommonLogger = log.New(os.Stdout, "[toyrpc] ", log.LstdFlags)
	ErrorLogger = log.New(os.Stdout, "[toyrpc ERROR] ", log.LstdFlags|log.Lshortfile)
}
