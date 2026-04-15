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
	"log"
	"net/http"

	_ "messager/docs"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	initDB()
	initRedis()

	r := mux.NewRouter()

	r.HandleFunc("/dialog/{user_id}/send",
		tokenRequired(sendMessage)).Methods("POST")

	r.HandleFunc("/dialog/{user_id}/list",
		tokenRequired(getDialog)).Methods("GET")

	// Swagger UI
	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	log.Println("dialog-service :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
