package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/redis/go-redis/v9"
)

func dialogKey(u1, u2 string) string {
	if u1 < u2 {
		return fmt.Sprintf("dialog:%s:%s", u1, u2)
	}
	return fmt.Sprintf("dialog:%s:%s", u2, u1)
}

func dialogVersionKey(u1, u2 string) string {
	if u1 < u2 {
		return fmt.Sprintf("dialog_version:%s:%s", u1, u2)
	}
	return fmt.Sprintf("dialog_version:%s:%s", u2, u1)
}

func invalidateDialog(ctx context.Context, u1, u2 string) {
	rdb.Incr(ctx, dialogVersionKey(u1, u2))
}

// @Summary Send message
// @Tags dialog
// @Security BearerAuth
// @Accept json
// @Param user_id path string true "Receiver"
// @Param message body Body true "text"
// @Success 200
// @Router /dialog/{user_id}/send [post]
func sendMessage(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	claims := r.Context().Value("user").(*Claims)
	toUser := mux.Vars(r)["user_id"]

	var b Body

	json.NewDecoder(r.Body).Decode(&b)

	_, err := dbWrite.Exec(`
		INSERT INTO messages (from_user, to_user, text)
		VALUES ($1, $2, $3)
	`, claims.User, toUser, b.Text)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// 🔥 инвалидируем кеш диалога
	invalidateDialog(ctx, claims.User, toUser)

	w.WriteHeader(200)
}

// @Summary Get dialog
// @Tags dialog
// @Security BearerAuth
// @Produce json
// @Param user_id path string true "User"
// @Success 200 {array} Message
// @Router /dialog/{user_id}/list [get]
func getDialog(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	claims := r.Context().Value("user").(*Claims)
	user2 := mux.Vars(r)["user_id"]

	version, err := rdb.Get(ctx, dialogVersionKey(claims.User, user2)).Result()
	if err != nil {
		version = "0"
	}

	cacheKey := fmt.Sprintf("%s:%s",
		dialogKey(claims.User, user2), version)

	// --- 1️⃣ Redis ---
	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return
	}

	// --- 2️⃣ DB ---
	rows, err := dbRead.Query(`
		SELECT id, from_user, to_user, text, created_at
		FROM messages
		WHERE LEAST(from_user, to_user) = LEAST($1, $2)
		  AND GREATEST(from_user, to_user) = GREATEST($1, $2)
		ORDER BY created_at ASC
	`, claims.User, user2)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var messages []Message

	for rows.Next() {
		var m Message
		rows.Scan(&m.ID, &m.FromUser, &m.ToUser, &m.Text, &m.CreatedAt)
		messages = append(messages, m)
	}

	data, _ := json.Marshal(messages)

	// --- 3️⃣ Redis save ---
	rdb.Set(ctx, cacheKey, data, 60*time.Second)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
