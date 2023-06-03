package main

import (
	"net/http"

	"gorm.io/gorm"
)

type Event struct {
	gorm.Model
	Title string
	Desc  string
	Quota uint
	Price uint
}

type BaseReturnPayload struct {
	Code    uint        `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type PayloadRequestBalance struct {
	UserId int `json:"user_id"`
}

type PayloadResponseBalance struct {
	Balance int64 `json:"balance"`
}

type HttpResponse struct {
	Status  int
	Body    []byte
	Error   error
	Headers http.Header
}
