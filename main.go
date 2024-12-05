package main

import (
	"bufio"
	"encoding/json"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type STATE int64

const (
	WAITING STATE = iota
	PLAYING
	END
)

var (
	filename      = "score.json"
	wordFile      = "word.txt"
	hangmanFile   = "hangman.txt"
	hangmanStages []string
	templates     *template.Template
	scoresMutex   sync.Mutex
)

type Score struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

type Scores struct {
	Scores []Score `json:"scores"`
}

type Game struct {
	PlayerName   string
	Word         string
	WordTest     []rune
	Decouverte   map[rune]bool
	Erreurs      int
	GameState    STATE
	PlayerWon    bool
	Propositions []string
	MaxAttempts  int
}

var games = make(map[string]*Game)
var gamesMutex sync.Mutex

func main() {
	rand.Seed(time.Now().UnixNano())

	var err error
	hangmanStages, err = loadHangmanStages(hangmanFile)
	if err != nil {
		log.Fatal(err)
	}

	templates = template.Must(template.ParseGlob("templates/*.html"))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/scores", scoresHandler)
	http.HandleFunc("/game", gameHandler)
	http.HandleFunc("/guess", guessHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("Serveur démarré sur http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Erreur lors du démarrage du serveur :", err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "index.html", nil)
}

func scoresHandler(w http.ResponseWriter, r *http.Request) {
	scores, err := readScores(filename)
	if err != nil {
		log.Println("Erreur lors de la lecture des scores :", err)
		scores = Scores{Scores: []Score{}}
	}

	// Trier les scores par ordre décroissant
	sort.Slice(scores.Scores, func(i, j int) bool {
		return scores.Scores[i].Score > scores.Scores[j].Score
	})

	templates.ExecuteTemplate(w, "scores.html", scores.Scores)
}

func gameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		pseudo := r.FormValue("pseudo")
		if pseudo == "" {
			http.Redirect(w, r, "/game", http.StatusSeeOther)
			return
		}

		gameID := strconv.Itoa(rand.Intn(1000000))
		game := startNewGame(pseudo)

		gamesMutex.Lock()
		games[gameID] = game
		gamesMutex.Unlock()

		http.Redirect(w, r, "/game?gameID="+gameID, http.StatusSeeOther)
		return
	}

	gameID := r.URL.Query().Get("gameID")

	var data map[string]interface{}

	if gameID != "" {
		gamesMutex.Lock()
		game, exists := games[gameID]
		gamesMutex.Unlock()

		if exists {
			data = map[string]interface{}{
				"GameID":       gameID,
				"Discovered":   string(game.WordTest),
				"AttemptsLeft": game.MaxAttempts - game.Erreurs,
				"HangmanStage": hangmanStages[game.Erreurs],
				"Message":      r.URL.Query().Get("message"),
				"PlayerName":   game.PlayerName,
			}
		} else {
			data = map[string]interface{}{
				"Message": "Partie non trouvée. Veuillez commencer une nouvelle partie.",
			}
		}
	} else {
		data = map[string]interface{}{
			"Message": r.URL.Query().Get("message"),
		}
	}

	templates.ExecuteTemplate(w, "game.html", data)
}

func guessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/game", http.StatusSeeOther)
		return
	}

	gameID := r.FormValue("gameID")
	input := r.FormValue("guess")

	gamesMutex.Lock()
	game, exists := games[gameID]
	gamesMutex.Unlock()

	if !exists {
		http.Redirect(w, r, "/game", http.StatusSeeOther)
		return
	}

	input = strings.TrimSpace(input)
	input = strings.ToLower(input)

	if input == "" {
		http.Redirect(w, r, "/game?gameID="+gameID, http.StatusSeeOther)
		return
	}

	if containsString(game.Propositions, input) {
		http.Redirect(w, r, "/game?gameID="+gameID+"&message=Déjà proposé", http.StatusSeeOther)
		return
	}

	game.Propositions = append(game.Propositions, input)

	if len(input) > 1 {
		if input == game.Word {
			game.PlayerWon = true
			game.WordTest = []rune(game.Word)
			game.GameState = END
		} else {
			game.Erreurs += 2
		}
	} else {
		letter := rune(input[0])
		if !unicode.IsLetter(letter) {
			http.Redirect(w, r, "/game?gameID="+gameID+"&message=Lettre invalide", http.StatusSeeOther)
			return
		}

		if game.Decouverte[letter] {
			http.Redirect(w, r, "/game?gameID="+gameID+"&message=Lettre déjà proposée", http.StatusSeeOther)
			return
		}

		game.Decouverte[letter] = true

		if strings.ContainsRune(game.Word, letter) {
			for i, l := range game.Word {
				if l == letter {
					game.WordTest[i] = letter
				}
			}
		} else {
			game.Erreurs++
		}
	}

	if !containsRune(game.WordTest, '_') {
		game.PlayerWon = true
		game.GameState = END
	} else if game.Erreurs >= game.MaxAttempts {
		game.GameState = END
	}

	if game.GameState == END {
		if game.PlayerWon {
			scoresMutex.Lock()
			scores, err := readScores(filename)
			if err != nil {
				log.Println("Erreur lors de la lecture des scores :", err)
				scores = Scores{Scores: []Score{}}
			}
			scoreOfPlayer, _ := getScore(scores, game.PlayerName)
			setScore(&scores, game.PlayerName, scoreOfPlayer+1)
			err = writeScores(filename, scores)
			if err != nil {
				log.Println("Erreur lors de l'écriture des scores :", err)
			}
			scoresMutex.Unlock()

			http.Redirect(w, r, "/game?message=Félicitations! Vous avez gagné!", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/game?message=Dommage! Vous avez perdu. Le mot était: "+game.Word, http.StatusSeeOther)
		}

		gamesMutex.Lock()
		delete(games, gameID)
		gamesMutex.Unlock()
	} else {
		gamesMutex.Lock()
		games[gameID] = game
		gamesMutex.Unlock()
		http.Redirect(w, r, "/game?gameID="+gameID, http.StatusSeeOther)
	}
}

func startNewGame(playerName string) *Game {
	mots, err := loadWords(wordFile)
	if err != nil {
		log.Println("Erreur lors du chargement des mots :", err)
		mots = []string{"erreur"}
	}

	index := rand.Intn(len(mots))
	word := strings.ToLower(mots[index])
	wordTest := make([]rune, len(word))
	for i := range word {
		wordTest[i] = '_'
	}

	revealCount := len(word)/2 - 1
	if revealCount < 0 {
		revealCount = 0
	}

	revealedIndices := make(map[int]bool)
	for len(revealedIndices) < revealCount {
		idx := rand.Intn(len(word))
		if !revealedIndices[idx] && wordTest[idx] == '_' {
			wordTest[idx] = rune(word[idx])
			revealedIndices[idx] = true
		}
	}

	return &Game{
		PlayerName:   playerName,
		Word:         word,
		WordTest:     wordTest,
		Decouverte:   make(map[rune]bool),
		Erreurs:      0,
		GameState:    PLAYING,
		PlayerWon:    false,
		Propositions: []string{},
		MaxAttempts:  len(hangmanStages) - 1,
	}
}

func loadWords(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var mots []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ligne := scanner.Text()
		mots = append(mots, ligne)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return mots, nil
}

func loadHangmanStages(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var stages []string
	scanner := bufio.NewScanner(file)
	var stageLines []string
	lineNumber := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNumber++

		if lineNumber == 1 {
			stages = append(stages, line)
			continue
		}

		if line == "" {
			continue
		}

		stageLines = append(stageLines, line)
		if len(stageLines) == 7 {
			stages = append(stages, strings.Join(stageLines, "\n"))
			stageLines = []string{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return stages, nil
}

func readScores(filename string) (Scores, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Scores{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var scores Scores
	err = decoder.Decode(&scores)
	if err != nil {
		return Scores{}, err
	}

	return scores, nil
}

func writeScores(filename string, scores Scores) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(scores)
	return err
}

func getScore(scores Scores, playerName string) (int, error) {
	for _, score := range scores.Scores {
		if score.Name == playerName {
			return score.Score, nil
		}
	}
	return 0, nil
}

func setScore(scores *Scores, playerName string, newScore int) {
	for i, score := range scores.Scores {
		if score.Name == playerName {
			scores.Scores[i].Score = newScore
			return
		}
	}
	scores.Scores = append(scores.Scores, Score{Name: playerName, Score: newScore})
}

func containsRune(slice []rune, r rune) bool {
	for _, v := range slice {
		if v == r {
			return true
		}
	}
	return false
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
