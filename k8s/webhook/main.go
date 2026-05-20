package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type AlertmanagerPayload struct {
	Version string  `json:"version"`
	Status  string  `json:"status"`
	Alerts  []Alert `json:"alerts"`
}

var deploymentToRestart = map[string][2]string{
	"Http500Spike": {"robot-shop", "catalogue"},
}

func restartDeployment(clientset *kubernetes.Clientset, namespace, deployment string) error {
	log.Printf("[heal] Restarting deployment %s/%s", namespace, deployment)
	patch := fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().UTC().Format(time.RFC3339),
	)
	_, err := clientset.AppsV1().
		Deployments(namespace).
		Patch(
			context.Background(),
			deployment,
			types.StrategicMergePatchType,
			[]byte(patch),
			metav1.PatchOptions{},
		)
	if err != nil {
		return fmt.Errorf("patch deployment %s/%s: %w", namespace, deployment, err)
	}
	log.Printf("[heal] Restart triggered for %s/%s", namespace, deployment)
	return nil
}

func healHandler(clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body error", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var payload AlertmanagerPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Printf("[heal] Bad JSON: %v", err)
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		log.Printf("[heal] Received %d alert(s), status=%s", len(payload.Alerts), payload.Status)
		for _, alert := range payload.Alerts {
			alertName := alert.Labels["alertname"]
			target, ok := deploymentToRestart[alertName]
			if !ok {
				log.Printf("[heal] No healing action for alert=%s", alertName)
				continue
			}
			if err := restartDeployment(clientset, target[0], target[1]); err != nil {
				log.Printf("[heal] ERROR: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	}
}

func alertLogHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()
	var payload AlertmanagerPayload
	if err := json.Unmarshal(body, &payload); err == nil {
		for _, a := range payload.Alerts {
			log.Printf("[alert] %s | status=%s", a.Labels["alertname"], a.Status)
		}
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "logged")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9091"
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("in-cluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("create k8s client: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/heal", healHandler(clientset))
	mux.HandleFunc("/alert", alertLogHandler)
	mux.HandleFunc("/health", healthHandler)
	log.Printf("[main] Webhook receiver listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("serve: %v", err)
	}
}