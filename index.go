package log_file

import (
	"github.com/chefsgo/log"
)

func Driver(ss ...string) log.Driver {
	s := ""
	if len(ss) > 0 {
		s = ss[0]
	}
	return &fileDriver{s}
}

func init() {
	log.Register("file", Driver("store/logs"))
}
