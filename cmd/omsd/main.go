package main

import (
	"flag"
	"fmt"
	"github.com/kardianos/service"
	log "github.com/sirupsen/logrus"
	"github.com/ssbeatty/oms/internal/config"
	"github.com/ssbeatty/oms/internal/models"
	"github.com/ssbeatty/oms/internal/server"
	"github.com/ssbeatty/oms/pkg/logger"
	"github.com/ssbeatty/oms/version"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

type App struct {
	conf *config.Conf
	sigs chan os.Signal
}

// NewApp create application
func NewApp(conf *config.Conf) *App {
	return &App{
		conf: conf,
	}
}

// Start application
func (a *App) Start(s service.Service) error {
	// run server
	srv := server.NewServer(a.conf)
	srv.Run()
	return nil
}

// Stop application
func (a *App) Stop(s service.Service) error {
	return nil
}

func main() {
	// flags & init conf

	configPath := flag.String("config", "", "path of config")
	act := flag.String("action", "", "install or uninstall")
	user := flag.String("user", "", "run with user")
	flag.Parse()

	var depends []string
	if runtime.GOOS != "windows" {
		depends = append(depends, "After=network.target")
	}
	var opt service.KeyValue
	switch runtime.GOOS {
	case "windows":
		opt = service.KeyValue{
			"StartType":              "automatic",
			"OnFailure":              "restart",
			"OnFailureDelayDuration": "5s",
			"OnFailureResetPeriod":   10,
		}
	case "linux":
		opt = service.KeyValue{
			"LimitNOFILE": 65000,
		}
	case "darwin":
		opt = service.KeyValue{
			"SessionCreate": true,
		}
	}

	appCfg := &service.Config{
		Name:         "omsd",
		DisplayName:  "omsd",
		Description:  "ssh & sftp manager service",
		UserName:     *user,
		Dependencies: depends,
		Option:       opt,
	}

	if *configPath != "" {
		abs, err := filepath.Abs(*configPath)
		if err == nil {
			appCfg.Arguments = []string{"--config", abs}
		}
	}

	conf, err := config.NewServerConfig(*configPath)
	if err != nil {
		panic(err)
	}

	if conf.App.Logger == "" || conf.App.Logger == "stdout" {
		logger.SetOutput(os.Stdout)
	} else {
		var logPath string

		if filepath.IsAbs(conf.App.Logger) {
			logPath = conf.App.Logger
		} else {
			logPath = filepath.Join(conf.App.DataPath, conf.App.Logger)
		}
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, fs.ModePerm)
		if err != nil {
			log.Error("打开日志文件失败!")
			return
		}
		logger.SetOutput(logFile)
	}

	log.Infof("当前版本: %s", version.Version)

	// init db
	db := conf.Db
	if err := models.InitModels(db.Dsn, db.DbName, db.UserName, db.PassWord, db.Driver, conf.App.DataPath); err != nil {
		panic(fmt.Sprintf("init db error: %v", err))
	}

	if conf.App.Mode == "dev" {
		logger.SetLevelAndFormat(logger.DebugLevel, &log.TextFormatter{})
	} else {
		logger.SetLevelAndFormat(logger.InfoLevel, &log.TextFormatter{})
	}

	app := NewApp(conf)
	sv, err := service.New(app, appCfg)
	if err != nil {
		panic(err)
	}

	switch *act {
	case "install":
		err = sv.Install()
		log.Infof("服务注册成功")
	case "uninstall":
		err = sv.Uninstall()
		log.Infof("服务取消成功")
	default:
		err = sv.Run()
		log.Info("程序退出")
	}
	if err != nil {
		panic(err)
	}

}
