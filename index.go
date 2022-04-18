package log_file

import (
	"github.com/chef-go/chef"
)

func Driver(ss ...string) chef.LogDriver {
	s := ""
	if len(ss) > 0 {
		s = ss[0]
	}
	return &fileLogDriver{s}
}

func init() {
	chef.Register("file", Driver("store/logs"))
}
