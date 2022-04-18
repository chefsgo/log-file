package log_file

import (
	"os"
	"path"
	"strings"
	"time"

	"github.com/chef-go/chef"
	"github.com/chef-go/util"
)

type (
	fileLogDriver struct {
		//store 驱动默认的存储路径
		store string
	}
	fileLogConnect struct {
		config  chef.LogConfig
		setting fileLogSetting
		writers map[chef.LogLevel]*FileWriter
	}
	fileLogSetting struct {
		//File 默认日志文件
		File string
		//LevelFiles 不同级别的日志文件
		LevelFiles map[chef.LogLevel]string
		// MaxSize 日志文件最大尺寸
		MaxSize int64
		//MaxLine 日志文件最大行
		MaxLine int64

		// DateSlice 日志文件按日期分片
		// 具体参考 checkSlice 方法
		DateSlice string
	}
)

func (driver *fileLogDriver) Connect(config chef.LogConfig) (chef.LogConnect, error) {
	//默认路径
	store := driver.store
	if vv, ok := config.Setting["store"].(string); ok && vv != "" {
		store = vv
	}

	_, e := os.Stat(store)
	if e != nil {
		//创建目录，如果不存在
		os.MkdirAll(store, 0700)
	}

	// 默认setting
	setting := fileLogSetting{
		LevelFiles: make(map[chef.LogLevel]string, 0),
		MaxSize:    1024 * 1024 * 100,
		MaxLine:    1000000,
		DateSlice:  "day",
	}

	levels := chef.LogLevels()
	for level, name := range levels {
		key := strings.ToLower(name)
		file := key + ".log"
		if vv, ok := config.Setting[key].(string); ok && vv != "" {
			setting.LevelFiles[level] = vv
		} else if vv, ok := config.Setting[key].(bool); ok && vv {
			setting.LevelFiles[level] = path.Join(store, file)
		} else {
			setting.LevelFiles[level] = path.Join(store, file)
		}
	}

	if vv, ok := config.Setting["output"].(string); ok && vv != "" {
		setting.File = vv
	} else if vv, ok := config.Setting["output"].(bool); ok && vv {
		setting.File = path.Join(store, "output.log")
	}

	//maxsize
	if vv, ok := config.Setting["maxsize"].(string); ok && vv != "" {
		size := util.ParseSize(vv)
		if size > 0 {
			setting.MaxSize = size
		}
	} else if vv, ok := config.Setting["maxsize"].(int64); ok && vv > 0 {
		setting.MaxSize = vv
	} else if vv, ok := config.Setting["weight"].(int64); ok && vv > 0 {
		setting.MaxSize = vv
	}

	//maxline
	if vv, ok := config.Setting["maxline"].(int64); ok && vv > 0 {
		setting.MaxLine = vv
	} else if vv, ok := config.Setting["height"].(int64); ok && vv > 0 {
		setting.MaxLine = vv
	}

	if vv, ok := config.Setting["slice"].(string); ok && vv != "" {
		setting.DateSlice = checkSlice(vv)
	}

	return &fileLogConnect{
		config: config, setting: setting,
	}, nil
}

//打开连接
func (connect *fileLogConnect) Open() error {

	writers := make(map[chef.LogLevel]*FileWriter, 0)
	if len(connect.setting.LevelFiles) > 0 {
		for level, filename := range connect.setting.LevelFiles {
			writer := newFileWriter(connect, filename)
			writer.init()
			writers[level] = writer
		}
	}
	if connect.setting.File != "" {
		writer := newFileWriter(connect, connect.setting.File)
		writer.init()
		writers[MAX_LEVEL] = writer
	}

	connect.writers = writers

	return nil
}

//关闭连接
func (connect *fileLogConnect) Close() error {
	//为了最后一条日志能正常输出，延迟一小会
	time.Sleep(time.Microsecond * 100)
	connect.Flush()
	return nil
}

// Write 写日志
// 可以考虑换成封闭好的协程库来执行并行任务
// 老代码搬运，暂时先这样
func (connect *fileLogConnect) Write(log chef.Log) error {
	var accessChan = make(chan error, 1)
	var levelChan = make(chan error, 1)

	if connect.setting.File != "" {
		go func() {
			accessFileWrite, ok := connect.writers[MAX_LEVEL]
			if !ok {
				accessChan <- nil
				return
			}
			err := accessFileWrite.write(log)
			if err != nil {
				accessChan <- err
				return
			}
			accessChan <- nil
		}()
	}

	if len(connect.setting.LevelFiles) != 0 {
		go func() {
			fileWrite, ok := connect.writers[log.Level]
			if !ok {
				levelChan <- nil
				return
			}
			err := fileWrite.write(log)
			if err != nil {
				levelChan <- err
				return
			}
			levelChan <- nil
		}()
	}

	var accessErr error
	var levelErr error
	if connect.setting.File != "" {
		accessErr = <-accessChan
	}
	if len(connect.setting.LevelFiles) != 0 {
		levelErr = <-levelChan
	}
	if accessErr != nil {
		return accessErr.(error)
	}
	if levelErr != nil {
		return levelErr.(error)
	}
	return nil
}

func (connect *fileLogConnect) Flush() {
	for _, writer := range connect.writers {
		writer.writer.Close()
	}
}
