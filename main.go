package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Structure de données pour stocker les informations sur les redirections et leur nombre d'utilisations
type InfoURL struct {
	URLOrigine string // URL d'origine
	URLCourte  string // URL raccourcie
	Compteur   int    // Compteur d'utilisations de l'URL raccourcie
}

var (
	db    *sql.DB    // Connexion à la base de données
	mutex sync.Mutex // Mutex pour la synchronisation des accès concurrents aux données
)

func main() {
	var err error

	// Connexion à la base de données MySQL sans spécifier de base de données
	tempDB, err := sql.Open("mysql", "root:@tcp(localhost:3306)/")
	if err != nil {
		log.Fatal(err)
	}
	defer tempDB.Close()

	// Création de la base de données si elle n'existe pas
	_, err = tempDB.Exec("CREATE DATABASE IF NOT EXISTS url_shortener_efrei")
	if err != nil {
		log.Fatal(err)
	}

	// Connexion à la base de données "url_shortener_efrei"
	db, err = sql.Open("mysql", "root:@tcp(localhost:3306)/url_shortener_efrei")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Création de la table si elle n'existe pas
	createTable := `
	CREATE TABLE IF NOT EXISTS urls (
		short_key VARCHAR(128) PRIMARY KEY,
		original_url TEXT,
		count INTEGER
	);
	`
	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}

	// Configuration des routes HTTP
	http.HandleFunc("/", handleFormulaire)
	http.HandleFunc("/short-url", handleRaccourcir) // Route pour afficher le raccourcis créé
	http.HandleFunc("/custom-url", handleRaccourcirPersonnalise) // Route pour afficher le raccourcis personnalisé
	http.HandleFunc("/short/", handleRedirection)   // Route d'accès au raccourcie
	http.HandleFunc("/list", handleListe)           // Route pour lister les raccourcies
	http.HandleFunc("/delete", handleSupprimer)     // Route pour la suppression des raccourcies
	http.HandleFunc("/customize", handleCustomizeForm) // Route pour afficher le formulaire personnalisé
	http.Handle("/style.css", http.FileServer(http.Dir("."))) // Route pour integrer le fichier de style


	fmt.Println("Starting server at http://localhost:8080")
	http.ListenAndServe(":8080", nil) // Route par défaut
	if err != nil {
		fmt.Println("Error starting server:", err)
	}

}

// handleFormulaire affiche le formulaire pour raccourcir une URL
func handleFormulaire(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("home.html")
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	if r.Method == http.MethodPost {
		http.Redirect(w, r, "/short-url", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, nil)
}

// handleCustomizeForm affiche le formulaire pour raccourcir une URL avec un raccourci personnalisé
func handleCustomizeForm(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("customize.html")
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, nil)
}

// handleRaccourcir crée une URL raccourcie à partir de l'URL d'origine
func handleRaccourcir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Opération invalide", http.StatusMethodNotAllowed)
		return
	}

	URLOrigine := r.FormValue("url")
	if URLOrigine == "" {
		http.Error(w, "Aucune URL présente", http.StatusBadRequest)
		return
	}

	URLCourte := genererCleRaccourcie()

	mutex.Lock()
	defer mutex.Unlock()

	// Insérer les données dans la base de données
	_, err := db.Exec("INSERT INTO urls (short_key, original_url, count) VALUES (?, ?, 0)", URLCourte, URLOrigine)
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	URLRaccourcie := fmt.Sprintf("http://localhost:8080/short/%s", URLCourte)

	tmpl, err := template.ParseFiles("short-url.html")
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	data := map[string]string{
		"URLOrigine":   URLOrigine,
		"URLRaccourcie": URLRaccourcie,
	}
	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, data)
}

// handleRaccourcirPersonnalise crée une URL raccourcie personnalisée à partir de l'URL d'origine
func handleRaccourcirPersonnalise(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Opération invalide", http.StatusMethodNotAllowed)
		return
	}

	URLOrigine := r.FormValue("url")
	URLCourte := r.FormValue("custom")

	if URLOrigine == "" || URLCourte == "" {
		http.Error(w, "Aucune URL ou clé personnalisée présente", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	// Vérifier si la clé personnalisée existe déjà
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM urls WHERE short_key = ?", URLCourte).Scan(&count)
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	if count > 0 {
		http.Error(w, "Cette clé personnalisée existe déjà", http.StatusBadRequest)
		return
	}

	// Insérer les données dans la base de données
	_, err = db.Exec("INSERT INTO urls (short_key, original_url, count) VALUES (?, ?, 0)", URLCourte, URLOrigine)
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	URLRaccourcie := fmt.Sprintf("http://localhost:8080/short/%s", URLCourte)

	tmpl, err := template.ParseFiles("short-url.html")
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	data := map[string]string{
		"URLOrigine":   URLOrigine,
		"URLRaccourcie": URLRaccourcie,
	}
	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, data)
}

// handleRedirection redirige vers l'URL d'origine correspondant à l'URL raccourcie
func handleRedirection(w http.ResponseWriter, r *http.Request) {
	URLRaccourcie := strings.TrimPrefix(r.URL.Path, "/short/")
	if URLRaccourcie == "" {
		http.Error(w, "Clé raccourcie manquante", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	// Récupérer les données de la base de données
	var URLOrigine string
	var Compteur int
	err := db.QueryRow("SELECT original_url, count FROM urls WHERE short_key = ?", URLRaccourcie).Scan(&URLOrigine, &Compteur)
	if err != nil {
		http.Error(w, "Clé raccourcie non présente", http.StatusNotFound)
		log.Println(err)
		return
	}

	// Incrémenter le compteur
	Compteur++
	_, err = db.Exec("UPDATE urls SET count = ? WHERE short_key = ?", Compteur, URLRaccourcie)
	if err != nil {
		log.Println(err)
	}

	// Redirection HTML
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<!DOCTYPE html><html><head><meta http-equiv=\"refresh\" content=\"0; url=%s\"></head><body></body></html>", URLOrigine)
}

// handleListe affiche une liste de toutes les redirections avec leur nombre d'utilisations
func handleListe(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT original_url, short_key, count FROM urls")
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer rows.Close()

	tmpl, err := template.ParseFiles("list.html")
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	var totalLiens int // Variable pour compter le nombre total de liens
	var urls []InfoURL

	for rows.Next() {
		var URLOrigine, URLCourte string
		var Compteur int
		if err := rows.Scan(&URLOrigine, &URLCourte, &Compteur); err != nil {
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		totalLiens++ // Incrémenter le compteur totalLiens
		urls = append(urls, InfoURL{
			URLOrigine: URLOrigine,
			URLCourte:  URLCourte,
			Compteur:   Compteur,
		})
	}

	data := map[string]interface{}{
		"TotalLiens": totalLiens,
		"URLs":       urls,
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, data)
}

// handleSupprimer supprime l'URL raccourcie de la base de données
func handleSupprimer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Opération invalide", http.StatusMethodNotAllowed)
		return
	}

	shortKey := r.FormValue("short_key")
	if shortKey == "" {
		http.Error(w, "Clé raccourcie manquante", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	_, err := db.Exec("DELETE FROM urls WHERE short_key = ?", shortKey)
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	http.Redirect(w, r, "/list", http.StatusSeeOther)
}

// Genere une clé raccourcie aléatoire
func genererCleRaccourcie() string {
	const caracteres = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
	const longueurCle = 8

	rand.Seed(time.Now().UnixNano())
	cleRaccourcie := make([]byte, longueurCle)
	for i := range cleRaccourcie {
		cleRaccourcie[i] = caracteres[rand.Intn(len(caracteres))]
	}
	return string(cleRaccourcie)
}
