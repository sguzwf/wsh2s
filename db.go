package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/empirefox/gotool/paas"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
)

var (
	DB *gorm.DB
	mu sync.Mutex
)

type DbFile struct {
	ID      string `gorm:"primary_key"`
	Content []byte
}

func SaveFileToDB(path string, contents []byte) (err error) {
	file := &DbFile{path, contents}
	err = DB.Create(file).Error
	if err != nil {
		err = DB.Save(file).Error
	}
	return
}

func LoadFileFromDB(path string) (contents []byte, err error) {
	var file DbFile
	if err = DB.First(&file, "id = ?", path).Error; err == gorm.ErrRecordNotFound {
		err = os.ErrNotExist
	}
	contents = file.Content
	return
}

func ConnectDBIfNot() {
	mu.Lock()
	if DB == nil {
		ConnectDB()
	}
	mu.Unlock()
}

func ConnectDB() {
	var err error

	if paas.Gorm.Url == "" {
		panic("DB_URL must be set if not in paas")
	}

	DB, err = gorm.Open(paas.Gorm.Dialect, paas.Gorm.Url)

	if err != nil {
		panic(fmt.Sprintf("No error should happen when connect database, but got %+v", err))
	}

	DB.DB().SetMaxIdleConns(4)
	DB.DB().SetMaxOpenConns(4)
}
