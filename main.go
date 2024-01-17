package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/joho/godotenv"
	"github.com/go-resty/resty"
)

// Person model
type Person struct {
	gorm.Model
	Name        string
	Surname     string
	Patronymic  string
	Age         int
	Gender      string
	Nationality string
}

var db *gorm.DB
var client = resty.New()

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Initialize database
	initDB()

	// Set up routes
	router := mux.NewRouter()
	router.HandleFunc("/people", getPeople).Methods("GET")
	router.HandleFunc("/people/{id}", getPerson).Methods("GET")
	router.HandleFunc("/people", createPerson).Methods("POST")
	router.HandleFunc("/people/{id}", updatePerson).Methods("PUT")
	router.HandleFunc("/people/{id}", deletePerson).Methods("DELETE")

	// Run the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func initDB() {
	var err error
	dbURL := os.Getenv("DATABASE_URL")
	db, err = gorm.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Automigrate the Person model
	db.AutoMigrate(&Person{})
}

func getPeople(w http.ResponseWriter, r *http.Request) {
	var people []Person
	db.Find(&people)
	respondJSON(w, http.StatusOK, people)
}

func getPerson(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	personID, err := strconv.Atoi(params["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid person ID")
		return
	}

	var person Person
	if err := db.First(&person, personID).Error; err != nil {
		respondError(w, http.StatusNotFound, "Person not found")
		return
	}

	respondJSON(w, http.StatusOK, person)
}

func createPerson(w http.ResponseWriter, r *http.Request) {
	var person Person
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&person); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	enrichPersonData(&person)

	db.Create(&person)

	respondJSON(w, http.StatusCreated, person)
}

func enrichPersonData(person *Person) {
	age, gender, nationality := getEnrichedData(person.Name)

	person.Age = age
	person.Gender = gender
	person.Nationality = nationality
}

func getEnrichedData(name string) (int, string, string) {
	age := getAgifyAge(name)
	gender := getGenderizeGender(name)
	nationality := getNationality(name)

	return age, gender, nationality
}

func getAgifyAge(name string) int {
	var response map[string]interface{}
	_, err := client.R().
		SetResult(&response).
		Get(fmt.Sprintf("%s/?name=%s", os.Getenv("AGIFY_API"), name))
	if err != nil {
		log.Printf("Error fetching Agify data: %v", err)
		return 0
	}

	age, _ := response["age"].(float64)
	return int(age)
}

func getGenderizeGender(name string) string {
	var response map[string]interface{}
	_, err := client.R().
		SetResult(&response).
		Get(fmt.Sprintf("%s/?name=%s", os.Getenv("GENDERIZE_API"), name))
	if err != nil {
		log.Printf("Error fetching Genderize data: %v", err)
		return ""
	}

	gender, _ := response["gender"].(string)
	return gender
}

func getNationality(name string) string {
	var response map[string]interface{}
	_, err := client.R().
		SetResult(&response).
		Get(fmt.Sprintf("%s/?name=%s", os.Getenv("NATIONALIZE_API"), name))
	if err != nil {
		log.Printf("Error fetching Nationalize data: %v", err)
		return ""
	}

	nationality, _ := response["country"].([]interface{})[0].(map[string]interface{})["country_id"].(string)
	return nationality
}

func updatePerson(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	personID, err := strconv.Atoi(params["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid person ID")
		return
	}

	var existingPerson Person
	if err := db.First(&existingPerson, personID).Error; err != nil {
		respondError(w, http.StatusNotFound, "Person not found")
		return
	}

	var updatedPerson Person
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&updatedPerson); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	existingPerson.Name = updatedPerson.Name
	existingPerson.Surname = updatedPerson.Surname
	existingPerson.Patronymic = updatedPerson.Patronymic

	enrichPersonData(&existingPerson)

	db.Save(&existingPerson)

	respondJSON(w, http.StatusOK, existingPerson)
}

func deletePerson(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	personID, err := strconv.Atoi(params["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid person ID")
		return
	}

	var person Person
	if err := db.First(&person, personID).Error; err != nil {
		respondError(w, http.StatusNotFound, "Person not found")
		return
	}

	db.Delete(&person)

	respondJSON(w, http.StatusOK, map[string]string{"message": "Person deleted successfully"})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, message string) {
	respondJSON(w, code, map[string]string{"error": message})
}