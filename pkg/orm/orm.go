package orm

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	DSN          string //数据库连接信息：root:123456@tcp(127.0.0.1:3306)/leonardo?charset=utf8mb4&parseTime=True&loc=Local
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  int
}

type DB struct { //自定义实现：目的是为了给DB增加自定义方法
	*gorm.DB
}

type ormLog struct { //ormLog实现了logger.Interface接口的成员函数
	LogLevel logger.LogLevel //LogLevel 的作用就是：保存当前日志级别，用于判断要不要打印。
}

func (l *ormLog) LogMode(level logger.LogLevel) logger.Interface {
	l.LogLevel = level
	return l
}

func (l *ormLog) Info(ctx context.Context, format string, v ...interface{}) {
	if l.LogLevel < logger.Info {
		return
	} //在做“日志级别过滤”，避免低级别日志被无意义打印。
	logx.WithContext(ctx).Infof(format, v...) //使用go-zero中的logx方法
}

func (l *ormLog) Warn(ctx context.Context, fromat string, v ...interface{}) {
	if l.LogLevel < logger.Warn {
		return
	}
	logx.WithContext(ctx).Infof(fromat, v...)
}

func (l *ormLog) Error(ctx context.Context, format string, v ...interface{}) {
	if l.LogLevel < logger.Error {
		return
	}
	logx.WithContext(ctx).Errorf(format, v...)
}

func (l *ormLog) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	logx.WithContext(ctx).WithDuration(elapsed).Infof("[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql)
}

func NewMysql(conf *Config) (*DB, error) {
	if conf.MaxIdleConns == 0 {
		conf.MaxIdleConns = 10
	}
	if conf.MaxOpenConns == 0 {
		conf.MaxOpenConns = 100
	}
	if conf.MaxLifetime == 0 {
		conf.MaxLifetime = 3600
	}
	db, err := gorm.Open(mysql.Open(conf.DSN), &gorm.Config{
		Logger: &ormLog{},
	}) //将自己的类型ormLog注入给GORM，GORM就会调用你的日志接口
	if err != nil {
		return nil, err
	}
	sdb, err := db.DB() //把 GORM 的 *gorm.DB 取出底层标准库连接对象 *sql.DB，用于后续设置参数
	if err != nil {
		return nil, err
	}
	sdb.SetMaxIdleConns(conf.MaxIdleConns)
	sdb.SetMaxOpenConns(conf.MaxOpenConns)
	sdb.SetConnMaxLifetime(time.Second * time.Duration(conf.MaxLifetime))

	err = db.Use(NewCustomePlugin()) //把“DB 的 trace + metrics 采集逻辑”挂到 GORM 生命周期里
	if err != nil {
		return nil, err
	}

	return &DB{DB: db}, nil
}

func MustNewMysql(conf *Config) *DB {
	db, err := NewMysql(conf)
	if err != nil {
		panic(err)
	}

	return db
}
