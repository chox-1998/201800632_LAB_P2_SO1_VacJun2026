package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	pb "go-deployment1/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Lo que va a mandar RUST_API
type PredictionRequest struct {
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeGoals int32  `json:"home_goals"`
	AwayGoals int32  `json:"away_goals"`
	Username  string `json:"username"`
	Timestamp string `json:"timestamp"`
}

var teamMap = map[string]pb.Teams{
	"GTM": pb.Teams_GTM,
	"MEX": pb.Teams_MEX,
	"BRA": pb.Teams_BRA,
	"ARG": pb.Teams_ARG,
	"ESP": pb.Teams_ESP,
}

func parseTeam(name string) pb.Teams {
	if t, ok := teamMap[strings.ToUpper(name)]; ok {
		return t
	}
	return pb.Teams_TEAMS_UNKNOWN
}

// -----------------------------------------------------------------------
// gRPC client wrapper
// -----------------------------------------------------------------------

type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.MatchPredictionServiceClient
}

// newGRPCClient dials the gRPC server (Deployment 2) and returns a client.
// The address is read from the GRPC_SERVER_ADDR env var
// (default: grpc-server-service:50051).
func newGRPCClient() (*GRPCClient, error) {
	addr := os.Getenv("GRPC_SERVER_ADDR")
	if addr == "" {
		addr = "grpc-server-service:50051"
	}

	log.Printf("[gRPC] Connecting to server at %s", addr)

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server: %w", err)
	}

	return &GRPCClient{
		conn:   conn,
		client: pb.NewMatchPredictionServiceClient(conn),
	}, nil
}

func (g *GRPCClient) SendPrediction(req PredictionRequest) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	grpcReq := &pb.MatchPredictionRequest{
		HomeTeam:  parseTeam(req.HomeTeam),
		AwayTeam:  parseTeam(req.AwayTeam),
		HomeGoals: req.HomeGoals,
		AwayGoals: req.AwayGoals,
		Username:  req.Username,
		Timestamp: req.Timestamp,
	}

	resp, err := g.client.SendPrediction(ctx, grpcReq)
	if err != nil {
		return "", fmt.Errorf("gRPC SendPrediction error: %w", err)
	}
	return resp.GetStatus(), nil
}

// -----------------------------------------------------------------------
// HTTP handlers
// -----------------------------------------------------------------------

type Server struct {
	grpc *GRPCClient
}

// handlePredict receives JSON from Rust, validates it, and forwards via gRPC.
func (s *Server) handlePredict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PredictionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[HTTP] Bad request: %v", err)
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Basic validation
	if req.HomeTeam == "" || req.AwayTeam == "" || req.Username == "" {
		http.Error(w, "Missing required fields: home_team, away_team, username", http.StatusBadRequest)
		return
	}
	if req.HomeTeam == req.AwayTeam {
		http.Error(w, "home_team and away_team must be different", http.StatusBadRequest)
		return
	}

	// Set default timestamp if omitted
	if req.Timestamp == "" {
		req.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	log.Printf("[HTTP] Received prediction: %s vs %s | %d-%d | user=%s",
		req.HomeTeam, req.AwayTeam, req.HomeGoals, req.AwayGoals, req.Username)

	grpcStatus, err := s.grpc.SendPrediction(req)
	if err != nil {
		log.Printf("[gRPC] Error forwarding prediction: %v", err)
		http.Error(w, "Failed to forward prediction to gRPC server", http.StatusBadGateway)
		return
	}

	log.Printf("[gRPC] Server responded with status: %s", grpcStatus)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": grpcStatus,
	})
}

// handleHealth is a liveness / readiness probe endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func main() {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	grpcClient, err := newGRPCClient()
	if err != nil {
		log.Fatalf("[FATAL] Cannot connect to gRPC server: %v", err)
	}
	defer grpcClient.conn.Close()

	srv := &Server{grpc: grpcClient}

	mux := http.NewServeMux()
	mux.HandleFunc("/predict", srv.handlePredict)
	mux.HandleFunc("/health", srv.handleHealth)

	log.Printf("[HTTP] go-deployment1 listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("[FATAL] HTTP server crashed: %v", err)
	}
}
