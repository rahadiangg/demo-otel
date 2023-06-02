package main

import (
	"log"
	"strconv"
	"time"

	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func initDB() *gorm.DB {
	dsn := "host=" + db_host + " user=postgres password=karcisDotCom dbname=karcis_com port=5432 sslmode=disable TimeZone=Asia/Jakarta"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Panicf("can't connect to DB: %s", err)
	}

	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		log.Panicf("error when using tracing otel gorm: %s", err)
	}

	sqlDb, _ := db.DB()

	mConn, err := strconv.Atoi(db_max_conn)
	if err != nil {
		log.Panicf("error when convert DB_MAX_CONN into integer")
	}

	sqlDb.SetMaxOpenConns(mConn) // default postgresql is 100
	sqlDb.SetMaxIdleConns(10)
	sqlDb.SetConnMaxLifetime(30 * time.Minute)

	// migrate
	if err := db.AutoMigrate(&Event{}); err != nil {
		log.Panicf("Migrate Event failed: %v", err)
	}

	// insert example data
	var data Event
	tx := db.First(&data, 1)
	if tx.Error != nil {
		if tx.Error.Error() == "record not found" {

			log.Print("record not found")

			dataInsert := Event{
				Title: "Coldplay Jakarta",
				Desc:  "Konser Coldplay pertama di Indonesia",
				Quota: 1000000,
				Price: 800000,
			}

			if result := db.Create(&dataInsert); result.Error != nil {
				log.Panicf("Insert example dataInsert failed: %v", err)
			}
		} else {
			log.Panicf("Error when get data ID 1: %v", tx.Error)
		}
	}

	return db
}
