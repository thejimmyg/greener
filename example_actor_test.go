package greener_test

import (
	"context"
	"fmt"
	"github.com/thejimmyg/greener"
	"net/http"
	"strings"
	"time"
)

// Quiz-related structs
type Quiz struct {
	Title     string     `json:"title"`
	Questions []Question `json:"questions"`
}

type CompletedQuiz struct {
	Title           string   `json:"title"`
	TeamAnswers     []Answer `json:"teamAnswers"`
	OfficialAnswers []Answer `json:"officialAnswers"`
}

type Question struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

type Answer struct {
	QuestionID int    `json:"questionId"`
	Value      string `json:"value"`
}

// LocalActor handles requests directly
type LocalActor struct{}

func (a *LocalActor) MarkQuiz(ctx context.Context, credentials string, completedQuiz CompletedQuiz) (int, error) {
	score := 0
	for i, answer := range completedQuiz.TeamAnswers {
		if completedQuiz.OfficialAnswers[i].Value == answer.Value {
			score += 1
		}
	}
	return score, nil // Assuming no errors occur in this simple computation
}

func (a *LocalActor) TakeQuiz(ctx context.Context, credentials string, quiz Quiz) ([]Answer, error) {
	var answers []Answer
	for _, question := range quiz.Questions {
		answers = append(answers, Answer{QuestionID: question.ID, Value: "4"})
	}
	return answers, nil // Assuming no errors occur in this simple computation
}

// RemoteActor has a client and server URL, and can send requests to the server.
type RemoteActor struct {
	ServerURL string
	Client    *http.Client
}

// Quiz methods that use RemoteCall
func (a *RemoteActor) MarkQuiz(ctx context.Context, credentials string, completedQuiz CompletedQuiz) (int, error) {
	return greener.ActorRemoteCall[CompletedQuiz, int](a.Client, a.ServerURL, "mark-quiz", ctx, credentials, completedQuiz)
}

func (a *RemoteActor) TakeQuiz(ctx context.Context, credentials string, quiz Quiz) ([]Answer, error) {
	return greener.ActorRemoteCall[Quiz, []Answer](a.Client, a.ServerURL, "take-quiz", ctx, credentials, quiz)
}

// HTTPServer wraps the LocalActor and implements http.Handler
type HTTPServer struct {
	actor *LocalActor
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	switch path {
	case "health":
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	case "mark-quiz":
		greener.ActorHandleCall[CompletedQuiz, int](w, r, s.actor.MarkQuiz)
	case "take-quiz":
		greener.ActorHandleCall[Quiz, []Answer](w, r, s.actor.TakeQuiz)
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func Example_actor() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2*time.Second))
	defer cancel()
	credentials := ""
	quiz := Quiz{
		Title: "Pub Quiz",
		Questions: []Question{
			{ID: 1, Text: "What is 2+2?"},
		},
	}

	localActor := &LocalActor{}
	answers, err := localActor.TakeQuiz(ctx, credentials, quiz)
	if err != nil {
		fmt.Println("Error taking quiz locally:", err)
	} else {
		fmt.Println("Local Answers:", answers)
	}

	completedQuiz := CompletedQuiz{
		Title:       "Pub Quiz",
		TeamAnswers: answers,
		OfficialAnswers: []Answer{
			{QuestionID: 1, Value: "4"},
		},
	}

	score, err := localActor.MarkQuiz(ctx, credentials, completedQuiz)
	if err != nil {
		fmt.Println("Error marking quiz locally:", err)
	} else {
		fmt.Println("Local Quiz Score:", score)
	}

	server := &HTTPServer{actor: localActor}
	fmt.Println("Server starting on port :8001...")
	go func() {
		// Start HTTP server
		if err := http.ListenAndServe("localhost:8001", server); err != nil {
			fmt.Println("Failed to start server:", err)
		}
	}()

	err = greener.PollForHealth("http://localhost:8001/health", 2*time.Second, 20*time.Millisecond)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	httpClient := &http.Client{
		Timeout: time.Second * 10, // 10-second timeout
	}

	remoteActor := RemoteActor{
		ServerURL: "http://localhost:8001",
		Client:    httpClient,
	}

	answers, err = remoteActor.TakeQuiz(ctx, credentials, quiz)
	if err != nil {
		fmt.Println("Error taking quiz remotely:", err)
	} else {
		fmt.Println("Remote Answers:", answers)
	}

	score, err = remoteActor.MarkQuiz(ctx, credentials, completedQuiz)
	if err != nil {
		fmt.Println("Error marking quiz remotely:", err)
	} else {
		fmt.Println("Remote Quiz Score:", score)
	}

	// Output: Local Answers: [{1 4}]
	// Local Quiz Score: 1
	// Server starting on port :8001...
	// Remote Answers: [{1 4}]
	// Remote Quiz Score: 1
}
