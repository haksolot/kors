package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

func sendRequest(id int) bool {
	query := fmt.Sprintf(`mutation { createResource(input: { typeName: "tool", initialState: "idle", metadata: { chaos_id: %d } }) { success } }`, id)
	body := map[string]string{"query": query}
	jb, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "http://localhost:8080/query", bytes.NewBuffer(jb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer system")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	b, _ := io.ReadAll(resp.Body)
	return bytes.Contains(b, []byte(`"success":true`))
}

func dockerAction(action, container string) {
	log.Printf("💀 CHAOS: %s %s...", action, container)
	cmd := exec.Command("docker", action, container)
	err := cmd.Run()
	if err != nil {
		log.Printf("Failed to %s %s: %v", action, container, err)
	}
}

func main() {
	var wg sync.WaitGroup
	stopTraffic := make(chan bool)

	// 1. Démarrer le trafic continu (10 requêtes par seconde)
	wg.Add(1)
	go func() {
		defer wg.Done()
		reqID := 0
		for {
			select {
			case <-stopTraffic:
				return
			default:
				reqID++
				success := sendRequest(reqID)
				if success {
					fmt.Print(".") // Success
				} else {
					fmt.Print("x") // Failure (Expected during chaos)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	time.Sleep(2 * time.Second)

	// 2. Déclencher le Chaos
	fmt.Println("\n--- INITIATING GIGA CHAOS ---")
	
	dockerAction("kill", "docker-nats-1")
	time.Sleep(2 * time.Second)
	
	dockerAction("kill", "docker-postgres-1")
	time.Sleep(5 * time.Second) // API should be completely failing now

	// 3. Rétablir l'infrastructure
	fmt.Println("\n--- RESTORING INFRASTRUCTURE ---")
	dockerAction("start", "docker-postgres-1")
	dockerAction("start", "docker-nats-1")

	// Laisser le temps à Postgres de démarrer et à l'API de se reconnecter
	time.Sleep(10 * time.Second)

	stopTraffic <- true
	wg.Wait()
	
	fmt.Println("\n--- CHAOS TEST FINISHED ---")
	fmt.Println("Vérifiez les logs de kors-api pour confirmer qu'elle n'a pas crashé.")
}
