package main

import "gorm.io/gorm"

type Event struct {
	gorm.Model
	Title string
	Desc  string
	Quota uint
	Price uint
}

type ReturnFormat struct {
	Code    uint        `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}
