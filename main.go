// hangman.go

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"sort"
	"strings"
	"unicode"
)

var filename string
var playerName string
var word string
var wordtest []rune
var hangmanStages []string

type STATE int64

const (
	WAITING STATE = iota
	PLAYING
	END
)

var GAME_STATE STATE

type Score struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

type Scores struct {
	Scores []Score `json:"scores"`
}

type GameState struct {
	PlayerName   string        `json:"player_name"`
	Word         string        `json:"word"`
	WordTest     []rune        `json:"word_test"`
	Decouverte   map[rune]bool `json:"decouverte"`
	Erreurs      int           `json:"erreurs"`
	GameState    STATE         `json:"game_state"`
	PlayerWon    bool          `json:"player_won"`
	Propositions []string      `json:"propositions"`
}

func readScores(filename string) (Scores, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Scores{}, err
	}
	defer file.Close()

	byteValue, _ := ioutil.ReadAll(file)

	var scores Scores
	err = json.Unmarshal(byteValue, &scores)
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

	jsonData, err := json.MarshalIndent(scores, "", "  ")
	if err != nil {
		return err
	}

	_, err = file.Write(jsonData)
	return err
}

func getScore(scores Scores, playerName string) (int, error) {
	for _, score := range scores.Scores {
		if score.Name == playerName {
			return score.Score, nil
		}
	}
	return 0, fmt.Errorf("Score non trouvé pour le joueur %s", playerName)
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

func loadHangmanStages(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Erreur lors de l'ouverture du fichier %s : %v", filename, err)
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
		return nil, fmt.Errorf("Erreur lors de la lecture du fichier %s : %v", filename, err)
	}
	return stages, nil
}

func main() {
	filename = "score.json"

	var startWith string
	flag.StringVar(&startWith, "startWith", "", "Commencer le jeu avec le fichier de sauvegarde spécifié")
	flag.Parse()

	var err error
	hangmanStages, err = loadHangmanStages("hangman.txt")
	if err != nil {
		fmt.Println(err)
		return
	}

	var mots []string
	file, err := os.Open("word.txt")
	if err != nil {
		fmt.Println("Erreur lors de l'ouverture du fichier des mots :", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ligne := scanner.Text()
		mots = append(mots, ligne)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Erreur lors de la lecture du fichier des mots :", err)
		return
	}

	if startWith != "" {
		savedGame, err := loadGame(startWith)
		if err != nil {
			fmt.Println("Erreur lors du chargement de la sauvegarde :", err)
			return
		}
		playerName = savedGame.PlayerName
		word = savedGame.Word
		wordtest = savedGame.WordTest
		GAME_STATE = savedGame.GameState

		scores, err := readScores(filename)
		if err != nil {
			fmt.Println("Erreur lors de la lecture du fichier des scores :", err)
			scores = Scores{Scores: []Score{}}
		}

		_, err2 := getScore(scores, playerName)
		if err2 != nil {
			newScore := Score{
				Name:  playerName,
				Score: 0,
			}
			scores.Scores = append(scores.Scores, newScore)
			err = writeScores(filename, scores)
			if err != nil {
				fmt.Println("Erreur lors de l'écriture dans le fichier des scores :", err)
			}
		}

		play(&savedGame.Decouverte, &savedGame.Erreurs, savedGame.PlayerWon, &savedGame.Propositions, mots)
	} else {
		fmt.Println("Bienvenue dans le jeu du Pendu")
		fmt.Println("Veuillez entrer votre pseudo :")
		fmt.Scan(&playerName)

		scores, err := readScores(filename)
		if err != nil {
			fmt.Println("Erreur lors de la lecture du fichier des scores :", err)
			scores = Scores{Scores: []Score{}}
		}

		_, err2 := getScore(scores, playerName)
		if err2 != nil {
			newScore := Score{
				Name:  playerName,
				Score: 0,
			}
			scores.Scores = append(scores.Scores, newScore)
			err = writeScores(filename, scores)
			if err != nil {
				fmt.Println("Erreur lors de l'écriture dans le fichier des scores :", err)
			}
		}

		for {
			welcome(scores, mots)
		}
	}
}

func welcome(scores Scores, mots []string) {
	fmt.Println("\nVeuillez choisir une option ci-dessous :")
	fmt.Println("1. Commencer une partie")
	fmt.Println("2. Voir les règles du jeu")
	fmt.Println("3. Voir les scores")
	fmt.Println("4. Quitter le jeu")

	var choice int
	fmt.Scan(&choice)
	switch choice {
	case 1:
		decouverte := make(map[rune]bool)
		erreurs := 0
		playerWon := false
		propositions := []string{}
		play(&decouverte, &erreurs, playerWon, &propositions, mots)
	case 2:
		fmt.Println("\nRègles du jeu :")
		fmt.Println("Un mot sera choisi aléatoirement parmi une liste prédéfinie.")
		fmt.Println("Vous avez un certain nombre de vies pour le deviner, correspondant au nombre d'étapes du dessin du pendu.")
		fmt.Println("Au début de chaque partie, certaines lettres seront déjà découvertes.")
		fmt.Println("Vous pouvez proposer soit une lettre, soit un mot (au moins deux caractères).")
		fmt.Println("Si vous proposez un mot et qu'il est incorrect, le compteur de tentatives diminue de 2.")
		fmt.Println("Vous ne pouvez pas proposer la même lettre deux fois.")
		fmt.Println("Vous pouvez taper 'STOP' à tout moment pour sauvegarder et quitter la partie.")
		fmt.Println("Bonne chance !\n")
	case 3:
		fmt.Println("\nScores actuels :")
		for _, sc := range scores.Scores {
			fmt.Println(sc.Name, ":", sc.Score)
		}
	case 4:
		fmt.Println("Merci d'avoir joué ! À bientôt.")
		os.Exit(0)
	default:
		fmt.Println("Choix invalide. Veuillez sélectionner une option de 1 à 4.")
	}
}

func play(decouverte *map[rune]bool, erreurs *int, playerWon bool, propositions *[]string, mots []string) {
	GAME_STATE = PLAYING
	tentativesMax := len(hangmanStages) - 1

	if word == "" {
		index := rand.Intn(len(mots))
		word = strings.ToLower(mots[index])
		wordtest = make([]rune, len(word))
		for i := range word {
			wordtest[i] = '_'
		}

		// Calcul du nombre de lettres à révéler
		revealCount := len(word)/2 - 1
		if revealCount < 0 {
			revealCount = 0
		}

		// Révélation aléatoire des lettres
		revealedIndices := make(map[int]bool)
		for len(revealedIndices) < revealCount {
			idx := rand.Intn(len(word))
			if !revealedIndices[idx] && wordtest[idx] == '_' {
				wordtest[idx] = rune(word[idx])
				revealedIndices[idx] = true
			}
		}
	}

	reader := bufio.NewReader(os.Stdin)

	for GAME_STATE == PLAYING {
		fmt.Println("\n-----------------------------")
		fmt.Println(hangmanStages[*erreurs])

		fmt.Printf("\nMot à deviner : ")
		for _, r := range wordtest {
			fmt.Printf("%c ", r)
		}
		fmt.Printf("\nLettres déjà devinées : ")
		letters := make([]rune, 0, len(*decouverte))
		for l := range *decouverte {
			letters = append(letters, l)
		}
		sort.Slice(letters, func(i, j int) bool { return letters[i] < letters[j] })
		for _, l := range letters {
			fmt.Printf("%c ", l)
		}
		fmt.Printf("\nTentatives restantes : %d\n", tentativesMax-*erreurs)
		fmt.Println("Veuillez entrer une lettre ou un mot (ou 'STOP' pour sauvegarder et quitter) :")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if strings.ToUpper(input) == "STOP" {
			err := saveGame("save.txt", decouverte, *erreurs, playerWon, propositions)
			if err != nil {
				fmt.Println("Erreur lors de la sauvegarde du jeu :", err)
			} else {
				fmt.Println("Jeu sauvegardé. À bientôt !")
			}
			os.Exit(0)
		}

		if containsString(*propositions, strings.ToLower(input)) {
			fmt.Println("Vous avez déjà proposé cette lettre ou ce mot.")
			continue
		}

		*propositions = append(*propositions, strings.ToLower(input))

		if len(input) > 1 {
			if strings.ToLower(input) == word {
				fmt.Println("\nFélicitations, vous avez deviné le mot !")
				fmt.Printf("Le mot était : %s\n", word)
				GAME_STATE = END
				playerWon = true
			} else {
				fmt.Println("Ce n'est pas le bon mot.")
				*erreurs += 2
			}
		} else if len(input) == 1 {
			if !unicode.IsLetter(rune(input[0])) {
				fmt.Println("Veuillez entrer une lettre valide.")
				continue
			}

			letter := rune(input[0])
			letter = unicode.ToLower(letter)

			if (*decouverte)[letter] {
				fmt.Println("Vous avez déjà deviné cette lettre.")
				continue
			}

			(*decouverte)[letter] = true

			if strings.ContainsRune(word, letter) {
				fmt.Println("Bonne réponse !")
				for i, l := range word {
					if l == letter {
						wordtest[i] = letter
					}
				}
			} else {
				fmt.Println("Mauvaise réponse.")
				*erreurs++
			}
		} else {
			fmt.Println("Entrée invalide.")
			continue
		}

		if !containsRune(wordtest, '_') {
			fmt.Println("\nFélicitations, vous avez gagné !")
			fmt.Printf("Le mot était : %s\n", word)
			GAME_STATE = END
			playerWon = true
		}

		if *erreurs >= tentativesMax {
			fmt.Println("\n-----------------------------")
			fmt.Println(hangmanStages[*erreurs])
			fmt.Println("\nVous avez perdu.")
			fmt.Printf("Le mot était : %s\n", word)
			GAME_STATE = END
		}
	}

	if GAME_STATE == END && playerWon {
		scores, err := readScores(filename)
		if err != nil {
			fmt.Println("Impossible de sauvegarder votre score !")
			return
		}
		scoreOfPlayer, _ := getScore(scores, playerName)
		setScore(&scores, playerName, scoreOfPlayer+1)
		err = writeScores(filename, scores)
		if err != nil {
			fmt.Println("Impossible de sauvegarder votre score !")
		}
	}

	word = ""
	wordtest = []rune{}
	GAME_STATE = WAITING
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

func saveGame(filename string, decouverte *map[rune]bool, erreurs int, playerWon bool, propositions *[]string) error {
	gameState := GameState{
		PlayerName:   playerName,
		Word:         word,
		WordTest:     wordtest,
		Decouverte:   *decouverte,
		Erreurs:      erreurs,
		GameState:    GAME_STATE,
		PlayerWon:    playerWon,
		Propositions: *propositions,
	}

	data, err := json.MarshalIndent(gameState, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filename, data, 0644)
	return err
}

func loadGame(filename string) (GameState, error) {
	var gameState GameState

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return gameState, err
	}

	err = json.Unmarshal(data, &gameState)
	if err != nil {
		return gameState, err
	}

	return gameState, nil
}
