package main

type BalanceRequest struct {
	UserId int `json:"user_id"`
}

type BalanceResponse struct {
	Balance int64 `json:"balance"`
}
