// @title Auth Service API
// @version 1.0
// @description Otus HighLoad
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name token
// @securityDefinitions.basic BasicAuth

package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "HL/docs"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger"
)

var rdb *redis.Client

var (
	dbWrite    *sql.DB
	dbRead     *sql.DB
	secretKey  = []byte("jndsifhvusdkhbfjdsfbgljdbgfvljdsgvjld")
	billingURL string
)

type User struct {
	ID        string
	Username  string `json:"username" example:"test1"`
	Password  string `json:"password,omitempty" example:"test1"`
	FirstName string `json:"first_name" example:"Aleksandr"`
	LastName  string `json:"second_name" example:"Pupkin"`
	Email     string `json:"email,omitempty" example:"test1@pupkin.ru"`
	Birthdate string `json:"birthdate,omitempty" example:"11-12-1988"`
	Gender    string `json:"gender,omitempty" example:"M"`
	Biography string `json:"biography,omitempty" example:"Music, photo and popcorn"`
	City      string `json:"city,omitempty" example:"SPB"`
	Phone     string `json:"phone,omitempty" example:"123456789"`
}

type Post struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type Claims struct {
	User  string `json:"user"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	jwt.RegisteredClaims
}

func env(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr: env("REDISCONN", "10.169.44.8:6379"),
	})
}

func initDB() {
	writeConn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		env("PGUSER", "postgres"),
		env("PGPASSWORD", "postgres"),
		env("PGHOST_WRITE", "10.169.44.8"),
		env("PGPORT_WRITE", "5000"),
		env("PGDBNAME", "auth"),
	)

	readConn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		env("PGUSER", "postgres"),
		env("PGPASSWORD", "postgres"),
		env("PGHOST_READ", "10.169.44.8"),
		env("PGPORT_READ", "5001"),
		env("PGDBNAME", "auth"),
	)

	var err error
	dbWrite, err = sql.Open("pgx", writeConn)
	if err != nil {
		log.Fatal(err)
	}

	dbRead, err = sql.Open("pgx", readConn)
	if err != nil {
		log.Fatal(err)
	}
}

func getFeedVersion(ctx context.Context, userID string) string {
	version, err := rdb.Get(ctx, "feed_version:"+userID).Result()
	if err != nil {
		return "0"
	}
	return version
}

func invalidateUserAndFriendsFeed(ctx context.Context, userID string) {
	// инвалидируем самого пользователя
	invalidateFeed(ctx, userID)

	// получаем друзей (подписчиков)
	rows, err := dbRead.Query(`
		SELECT user_id FROM friends WHERE friend_id=$1`,
		userID,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			continue
		}
		invalidateFeed(ctx, uid)
	}
}

func invalidateFeed(ctx context.Context, userID string) {
	rdb.Incr(ctx, "feed_version:"+userID)
}

func invalidateTwoFeeds(ctx context.Context, u1, u2 string) {
	invalidateFeed(ctx, u1)
	invalidateFeed(ctx, u2)
}

func getProfile(username string) (*User, error) {
	row := dbRead.QueryRow(`
		SELECT
			password,
			COALESCE(email, ''),
			COALESCE(phone, '')
			FROM users WHERE username=$1`, username)

	u := &User{Username: username}
	err := row.Scan(&u.Password, &u.Email, &u.Phone)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func tokenRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		tokenStr := r.Header.Get("token")
		if tokenStr == "" {
			http.Error(w, "token is missing", 403)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(
			tokenStr,
			claims,
			func(t *jwt.Token) (interface{}, error) {
				return secretKey, nil
			},
		)

		if err != nil || !token.Valid {
			http.Error(w, "token is invalid", 403)
			return
		}

		ctx := context.WithValue(r.Context(), "user", claims)
		next(w, r.WithContext(ctx))
	}
}

// hello godoc
// @Summary Hello endpoint
// @Tags system
// @Success 200
// @Router / [get]
func hello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, World!"))
}

// getUser godoc
// @Summary Get user
// @Tags user
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 200 {object} User
// @Router /user/get/{username} [get]
func getUser(w http.ResponseWriter, r *http.Request) {
	//	claims := r.Context().Value("user").(*Claims)

	username := mux.Vars(r)["username"]

	row := dbRead.QueryRow(`
		SELECT 
			username,
			firstname,
			lastname,
			COALESCE(email, ''),
			COALESCE(birthdate, ''),
			COALESCE(biography, ''),
			COALESCE(city, ''),
			COALESCE(phone, '')
			COALESCE(gender, '')
		FROM users WHERE username=$1`, username)

	var u User
	err := row.Scan(&u.Username, &u.FirstName, &u.LastName, &u.Email, &u.Birthdate, &u.Biography, &u.City, &u.Phone, &u.Gender)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(u)
}

// deleteUser godoc
// @Summary Delete user
// @Tags user
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 204
// @Router /user/delete/{username} [delete]
func deleteUser(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("user").(*Claims)

	if claims.User != "admin" {
		http.Error(w, "token is invalid", 403)
		return
	}

	username := mux.Vars(r)["username"]

	_, err := dbWrite.Exec(`DELETE FROM users WHERE username=$1`, username)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("user").(*Claims)

	var u User
	json.NewDecoder(r.Body).Decode(&u)

	_, err := dbWrite.Exec(`
		UPDATE users SET
			firstname=$1,
			lastname=$2,
			email=$3,
			phone=$4
		WHERE username=$5`,
		u.FirstName, u.LastName, u.Email, u.Phone, claims.User)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

// createUser godoc
// @Summary Create user
// @Tags user
// @Security BearerAuth
// @Accept json
// @Param user body User true "User"
// @Success 201
// @Router /user/register [post]
func createUser(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("user").(*Claims)

	if claims.User != "admin" {
		http.Error(w, "token is invalid", 403)
		return
	}

	var u User
	json.NewDecoder(r.Body).Decode(&u)

	_, err := dbWrite.Exec(`
		INSERT INTO users
		(username,password,firstname,lastname,email,phone,biography,birthdate,city,gender)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		u.Username, u.Password, u.FirstName, u.LastName, u.Email, u.Phone, u.Biography, u.Birthdate, u.City, u.Gender)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// billing request
	payload, _ := json.Marshal(map[string]string{
		"username": u.Username,
	})

	req, _ := http.NewRequest(
		"POST",
		"http://"+billingURL+"/api/v1/user",
		strings.NewReader(string(payload)),
	)

	req.Header.Set("Content-Type", "application/json")
	req.URL.RawQuery = r.URL.RawQuery

	http.DefaultClient.Do(req)

	w.WriteHeader(201)
	w.Write([]byte(u.Username))
}

// health godoc
// @Summary Health check
// @Tags system
// @Success 200
// @Router /health [get]
func health(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status":"OK"}`))
}

// searchUser godoc
// @Summary Search users
// @Tags user
// @Security BearerAuth
// @Param first_name query string true "First name"
// @Param last_name query string true "Last name"
// @Success 200 {array} User
// @Router /user/search [get]
func searchUser(w http.ResponseWriter, r *http.Request) {

	firstName := r.URL.Query().Get("first_name")
	lastName := r.URL.Query().Get("last_name")

	if firstName == "" || lastName == "" {
		http.Error(w, "first_name and last_name required", http.StatusBadRequest)
		return
	}

	rows, err := dbRead.Query(`
		SELECT
		    id,
			username,
			firstname,
			lastname,
			COALESCE(email, ''),
			COALESCE(birthdate, ''),
			COALESCE(biography, ''),
			COALESCE(city, ''),
			COALESCE(phone, '')
		FROM users
		WHERE firstname ILIKE $1
		  AND lastname ILIKE $2
		LIMIT 50`,
		firstName+"%",
		lastName+"%",
	)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	result := []User{}

	for rows.Next() {
		var u User

		err := rows.Scan(
			&u.ID,
			&u.Username,
			&u.FirstName,
			&u.LastName,
			&u.Email,
			&u.Birthdate,
			&u.Biography,
			&u.City,
			&u.Phone,
		)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		result = append(result, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// @Summary Add friend
// @Tags friend
// @Security BearerAuth
// @Param user_id query string true "Friend user ID"
// @Success 200
// @Router /friend/set [post]
func setFriend(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("user").(*Claims)

	friendID := r.URL.Query().Get("user_id")
	if friendID == "" {
		http.Error(w, "user_id required", 400)
		return
	}

	if friendID == claims.User {
		http.Error(w, "cannot add yourself", 400)
		return
	}

	// INSERT (игнорируем дубликаты)
	_, err := dbWrite.Exec(`
		INSERT INTO friends (username, friendname)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`,
		claims.User,
		friendID,
	)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	ctx := context.Background()
	invalidateTwoFeeds(ctx, claims.User, friendID)

	w.WriteHeader(http.StatusOK)
}

// @Summary Delete friend
// @Tags friend
// @Security BearerAuth
// @Param user_id query string true "Friend user ID"
// @Success 204
// @Router /friend/delete [delete]
func deleteFriend(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("user").(*Claims)

	friendID := r.URL.Query().Get("user_id")
	if friendID == "" {
		http.Error(w, "user_id required", 400)
		return
	}

	_, err := dbWrite.Exec(`
		DELETE FROM friends
		WHERE username=$1 AND friendname=$2`,
		claims.User,
		friendID,
	)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	ctx := context.Background()
	invalidateTwoFeeds(ctx, claims.User, friendID)

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Create post
// @Tags post
// @Security BearerAuth
// @Accept json
// @Param post body Post true "Post"
// @Success 201
// @Router /post/create [post]
func createPost(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("user").(*Claims)

	var p Post
	json.NewDecoder(r.Body).Decode(&p)

	err := dbWrite.QueryRow(`
		INSERT INTO posts(username, text)
		VALUES ($1, $2)
		RETURNING id, created_at`,
		claims.User, p.Text,
	).Scan(&p.ID, &p.CreatedAt)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	ctx := context.Background()
	invalidateUserAndFriendsFeed(ctx, claims.User)

	p.Username = claims.User

	json.NewEncoder(w).Encode(p)
}

// @Summary Get post
// @Tags post
// @Security BearerAuth
// @Param id query int true "Post ID"
// @Success 200 {object} Post
// @Router /post/get [get]
func getPost(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	var p Post

	err := dbRead.QueryRow(`
		SELECT id, username, text, created_at
		FROM posts WHERE id=$1`, id).
		Scan(&p.ID, &p.Username, &p.Text, &p.CreatedAt)

	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	json.NewEncoder(w).Encode(p)
}

// @Summary Update post
// @Tags post
// @Security BearerAuth
// @Accept json
// @Param post body Post true "Post"
// @Success 200
// @Router /post/update [put]
func updatePost(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("user").(*Claims)

	var p Post
	json.NewDecoder(r.Body).Decode(&p)

	res, err := dbWrite.Exec(`
		UPDATE posts
		SET text=$1
		WHERE id=$2 AND username=$3`,
		p.Text, p.ID, claims.User)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		http.Error(w, "not found or forbidden", 403)
		return
	}

	ctx := context.Background()
	invalidateUserAndFriendsFeed(ctx, claims.User)

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Delete post
// @Tags post
// @Security BearerAuth
// @Param id query int true "Post ID"
// @Success 204
// @Router /post/delete [delete]
func deletePost(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("user").(*Claims)
	id := r.URL.Query().Get("id")

	res, err := dbWrite.Exec(`
		DELETE FROM posts
		WHERE id=$1 AND username=$2`,
		id, claims.User)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		http.Error(w, "not found or forbidden", 403)
		return
	}

	ctx := context.Background()
	invalidateUserAndFriendsFeed(ctx, claims.User)

	w.WriteHeader(http.StatusNoContent)
}

// feed godoc
// @Summary Get user feed
// @Description Returns paginated feed of posts from friends
// @Tags post
// @Security BearerAuth
// @Produce json
// @Param offset query int false "Offset" example(0)
// @Param limit query int false "Limit" example(10)
// @Success 200 {array} Post
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal server error"
// @Router /post/feed [get]
func feed(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	start := time.Now()
	claims := r.Context().Value("user").(*Claims)

	// --- параметры ---
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	log.Printf("[feed] user=%s offset=%d limit=%d", claims.User, offset, limit)

	// --- version ---
	version := getFeedVersion(ctx, claims.User)

	// --- cache key ---
	cacheKey := fmt.Sprintf("feed:%s:%s:%d:%d",
		claims.User, version, offset, limit)

	// --- 1️⃣ пробуем Redis ---
	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		log.Printf("[feed] total took with Redis only=%s", time.Since(start))
		return
	}

	log.Printf("[feed] cache MISS key=%s", cacheKey)

	// --- 2️⃣ защита от cache stampede ---
	lockKey := "lock:" + cacheKey

	ok, _ := rdb.SetNX(ctx, lockKey, 1, 5*time.Second).Result()
	if !ok {
		time.Sleep(50 * time.Millisecond)
		feed(w, r)
		return
	}
	defer rdb.Del(ctx, lockKey)

	dbStart := time.Now()

	// --- 3️⃣ запрос в БД ---
	rows, err := dbRead.Query(`
		SELECT p.id, p.username, p.text, p.created_at
		FROM posts p
		JOIN friends f ON f.friendname = p.username
		WHERE f.username = $1
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3`,
		claims.User,
		limit,
		offset,
	)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var posts []Post

	for rows.Next() {
		var p Post
		err := rows.Scan(&p.ID, &p.Username, &p.Text, &p.CreatedAt)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		posts = append(posts, p)
	}

	log.Printf("[feed] db query took=%s", time.Since(dbStart))

	// --- 4️⃣ сериализация ---
	data, _ := json.Marshal(posts)

	// --- 5️⃣ кешируем ---
	err = rdb.Set(ctx, cacheKey, data, 30*time.Second).Err()
	if err != nil {
		log.Printf("[feed] redis set error: %v", err)
	}

	// --- 6️⃣ ответ ---
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)

	log.Printf("[feed] total took with DB=%s", time.Since(start))
}

// login godoc
// @Summary Login user
// @Tags auth
// @Produce json
// @Security BasicAuth
// @Success 200 {object} map[string]string
// @Failure 401
// @Router /login [post]
func login(w http.ResponseWriter, r *http.Request) {

	auth := r.Header.Get("Authorization")
	if auth == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="Login Required"`)
		http.Error(w, "you need be authorised", 401)
		return
	}

	payload, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	parts := strings.Split(string(payload), ":")

	username := parts[0]
	password := strings.TrimSpace(parts[1])

	profile, err := getProfile(username)
	if err != nil || profile.Password != password {
		http.Error(w, "you are not authorised", 401)
		return
	}

	claims := Claims{
		User:  username,
		Email: profile.Email,
		Phone: profile.Phone,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(50 * time.Minute)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(secretKey)

	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenStr,
	})
}

func main() {

	initDB()

	initRedis()

	r := mux.NewRouter()

	r.HandleFunc("/", hello).Methods("GET")
	r.HandleFunc("/health", health).Methods("GET")
	r.Handle("/metrics", promhttp.Handler()).Methods("GET")

	r.HandleFunc("/login", login)
	r.HandleFunc("/user/get/{username}", tokenRequired(getUser)).Methods("GET")
	r.HandleFunc("/user/delete/{username}", tokenRequired(deleteUser)).Methods("DELETE")
	r.HandleFunc("/user/search", tokenRequired(searchUser)).Methods("GET")
	//	r.HandleFunc("/user/update/{username}", tokenRequired(updateUser)).Methods("PUT")
	r.HandleFunc("/user/register", tokenRequired(createUser)).Methods("POST")

	r.HandleFunc("/friend/set", tokenRequired(setFriend)).Methods("POST")
	r.HandleFunc("/friend/delete", tokenRequired(deleteFriend)).Methods("DELETE")
	r.HandleFunc("/post/create", tokenRequired(createPost)).Methods("POST")
	r.HandleFunc("/post/get", tokenRequired(getPost)).Methods("GET")
	r.HandleFunc("/post/update", tokenRequired(updatePost)).Methods("PUT")
	r.HandleFunc("/post/delete", tokenRequired(deletePost)).Methods("DELETE")
	r.HandleFunc("/post/feed", tokenRequired(feed)).Methods("GET")

	// Swagger UI
	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	log.Println("Server started :8000")
	log.Fatal(http.ListenAndServe(":8000", r))
}
