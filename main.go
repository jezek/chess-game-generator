package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/andrewbackes/chess/game"
	"github.com/andrewbackes/chess/position"
	"github.com/andrewbackes/chess/position/move"
)

// Stores games with half-moves closest to key value.
var gamesOfLength = map[int]*game.Game{
	10:  nil,
	25:  nil,
	50:  nil,
	100: nil,
	250: nil,
	500: nil,
	750: nil,
}

func main() {
	// Number games to be generated with seeds from 0 to noSearches-1, to find games of certain length.
	// Note: Tried to 10000, but for following gamesOfLength keys, 1500 is enough.
	noSearches := 10000

	// Get generated games from storage.
	storageFileName := "./generateStorage.txt"
	f, err := os.OpenFile(storageFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Error opening/creating storage file: %v", err)
	}
	scanner := bufio.NewScanner(f)
	startIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Printf("Error parsing storage line %d: %v", startIndex, err)
			log.Fatalf("Storage file \"%s\" is corrupt. Repair or remove it and restart tests.", storageFileName)
		}
		moves := parts[1:]
		if n != len(moves) {
			log.Printf("Error quick validating storage line %d: %s", startIndex, fmt.Sprint("number of moves ", n, " does not correspond to umber of SAN moves ", len(moves)))
			log.Fatalf("Storage file \"%s\" is corrupt. Repair or remove it and restart tests.", storageFileName)
		}
		g := &game.Game{
			Tags: map[string]string{
				"#":        fmt.Sprint(startIndex),
				"sanMoves": strings.Join(moves, " "),
			},
			Positions: make([]*position.Position, 0, n+1),
		}
		addToGamesOfLength(g)
		startIndex += 1
		if startIndex >= noSearches {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading storage file: %v", err)
	}

	// Generate new games and store them.
	writer := bufio.NewWriter(f)
	for i := startIndex; i < noSearches; i += 1 {
		log.Print("Generating game with seed #", i)
		g, err := generateRandomGame(i)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("GameStatus after %d half-moves: %v", len(g.Positions)-1, g.Status())
		storeGame(writer, g)
		if err := f.Sync(); err != nil {
			log.Printf("Error syncing storage to disk: %v", err)
			return
		}
	}
	f.Close()

	// Compute results and save to file.
	resultFileName := fmt.Sprintf("./generated_%d.txt", noSearches)
	f, err = os.OpenFile(resultFileName, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Printf("Error creating result file: %v", err)
	} else {
		defer f.Close()
	}
	writer = bufio.NewWriter(f)
	log.Printf("Writing results to: %s", resultFileName)
	lengths := make([]int, 0, len(gamesOfLength))
	for l, _ := range gamesOfLength {
		lengths = append(lengths, l)
	}
	sort.Slice(lengths, func(i, j int) bool {
		return lengths[i] < lengths[j]
	})
	for _, l := range lengths {
		g := gamesOfLength[l]
		if len(g.Positions) == 0 {
			seed, err := strconv.Atoi(g.Tags["#"])
			if err != nil {
				log.Printf("Error getting seed from game tags for game of length %d: %v", l, err)
				continue
			}
			ng, err := generateRandomGame(seed)
			if err != nil {
				log.Print(err)
			}
			ng.Tags = g.Tags
			g = ng
		}
		sanMoves := getSANMoves(g)
		if g.Tags["sanMoves"] != "" {
			genSanMoves := strings.Join(sanMoves, " ")
			if g.Tags["sanMoves"] != genSanMoves {
				log.Print("Storage moves:   ", g.Tags["sanMoves"])
				log.Print("Generated moves: ", genSanMoves)
				log.Printf("Moves for game #%s loaded from storage are not equal to generated moves", g.Tags["#"])
			}
		}
		log.Printf("Target length: %d | Random game #%s | half moves: %d", l, g.Tags["#"], len(g.Positions)-1)
		if _, err := writer.WriteString(fmt.Sprintf("{\n\t\"Random-game-#%s_half-moves-%d_target-%d\", \"\",\n\t%#v,\n},\n", g.Tags["#"], len(g.Positions)-1, l, sanMoves)); err != nil {
			log.Printf("Error writing result for length %d to result file: %v", l, err)
		}
	}
}

func getGameLength(g *game.Game) int {
	if len(g.Positions) == 0 { // Games with 0 length are from storage and the length is stored in capacity of Game.Positions slice.
		return cap(g.Positions) - 1
	}
	return len(g.Positions) - 1
}

func dist(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

func addToGamesOfLength(g *game.Game) {
	n := getGameLength(g)
	for l, lg := range gamesOfLength {
		if lg == nil {
			gamesOfLength[l] = g
			continue
		}
		lgn := getGameLength(lg)
		if dist(l, n) < dist(l, lgn) {
			gamesOfLength[l] = g
		}
	}
}

func generateRandomGame(seed int) (*game.Game, error) {
	g, gs, err := game.New(), game.InProgress, error(nil)
	g.Tags["#"] = fmt.Sprint(seed)
	rnd := rand.New(rand.NewSource(int64(seed)))
	for gs == game.InProgress {
		movesMap := g.LegalMoves()
		movesSlice := []move.Move{}
		for key := range movesMap {
			movesSlice = append(movesSlice, key)
		}
		sort.Slice(movesSlice, func(i, j int) bool {
			return movesSlice[i].String() < movesSlice[j].String()
		})

		gs, err = g.MakeMove(movesSlice[rnd.Intn(len(movesSlice))])
		if err != nil {
			return nil, err
		}
	}
	return g, nil
}

func getSANMoves(g *game.Game) []string {
	sanMoves := make([]string, 0, len(g.Positions)-1)
	for i := range g.Positions {
		if g.Positions[i].LastMove != move.Null {
			sanMoves = append(sanMoves, g.Positions[i-1].SAN(g.Positions[i].LastMove))
		}
	}
	return sanMoves
}

func storeGame(writer *bufio.Writer, g *game.Game) {
	_, err := writer.WriteString(fmt.Sprint(len(g.Positions)-1, " ", strings.Join(getSANMoves(g), " "), "\n"))
	if err != nil {
		log.Printf("Error storing game to storage: %v", err)
	}
	err = writer.Flush()
	if err != nil {
		log.Printf("Error flushing game to storage writer: %v", err)
	}
	addToGamesOfLength(g)
}
