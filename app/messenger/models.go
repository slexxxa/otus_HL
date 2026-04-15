package main

import "time"

type Message struct {
	ID        int       `json:"id"`
	FromUser  string    `json:"from_user"`
	ToUser    string    `json:"to_user"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type Body struct {
	Text string `json:"text"`
}
