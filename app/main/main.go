// @title Auth Service API
// @version 1.0
// @description Otus HighLoad
// @host 127.0.0.1:8000
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
	"strings"
	"time"

	_ "HL/docs"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
)

var (
	db         *sql.DB
	secretKey  = []byte("jndsifhvusdkhbfjdsfbgljdbgfvljdsgvjld")
	billingURL string
)

type User struct {
	Username  string `json:"username" example:"test1"`
	Password  string `json:"password,omitempty" example:"test1"`
	FirstName string `json:"first_name" example:"Aleksandr"`
	LastName  string `json:"second_name" example:"Pupkin"`
	Email     string `json:"email" example:"test1@pupkin.ru"`
	Birthdate string `json:"birthdate" example:"11-12-1988"`
	Gender    string `json:"gender" example:"M"`
	Biography string `json:"biography" example:"Music, photo and popcorn"`
	City      string `json:"city" example:"SPB"`
	Phone     string `json:"phone" example:"123456789"`
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

func initDB() {
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		env("PGUSER", "postgres"),
		env("PGPASSWORD", "12345678"),
		env("PGHOST", "10.169.44.8"),
		env("PGPORT", "5432"),
		env("PGDBNAME", "auth"),
	)

	var err error
	db, err = sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal(err)
	}
}

func getProfile(username string) (*User, error) {
	row := db.QueryRow(`
		SELECT password, email, phone
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

	row := db.QueryRow(`
		SELECT username,firstname,lastname,email,phone,biography,birthdate,city,gender
		FROM users WHERE username=$1`, username)

	var u User
	err := row.Scan(&u.Username, &u.FirstName, &u.LastName, &u.Email, &u.Phone, &u.Biography, &u.Birthdate, &u.City, &u.Gender)
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

	_, err := db.Exec(`DELETE FROM users WHERE username=$1`, username)
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

	_, err := db.Exec(`
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

	_, err := db.Exec(`
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

	r := mux.NewRouter()

	r.HandleFunc("/", hello).Methods("GET")
	r.HandleFunc("/health", health).Methods("GET")
	r.Handle("/metrics", promhttp.Handler()).Methods("GET")

	r.HandleFunc("/login", login)
	r.HandleFunc("/user/get/{username}", tokenRequired(getUser)).Methods("GET")
	r.HandleFunc("/user/delete/{username}", tokenRequired(deleteUser)).Methods("DELETE")
	//	r.HandleFunc("/user/update/{username}", tokenRequired(updateUser)).Methods("PUT")
	r.HandleFunc("/user/register", tokenRequired(createUser)).Methods("POST")

	// Swagger UI
	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	log.Println("Server started :8000")
	log.Fatal(http.ListenAndServe(":8000", r))
}
