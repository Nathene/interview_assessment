package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Nathene/interview_assessment/auth"
	"github.com/Nathene/interview_assessment/database"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	// Public endpoints
	e.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, "Hello, world!")
	})

	// Development login endpoint - assigns role based on query param
	// Will be replaced with oath eventually..
	e.GET("/dev-login", func(c echo.Context) error {
		role := c.QueryParam("role")
		if role == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Role is required",
			})
		}

		token, err := auth.GenerateToken(
			uuid.New().String(), // random userID
			"dev@example.com",
			"Dev User",
			role,
		)
		if err != nil {
			return echo.ErrInternalServerError
		}

		return c.JSON(http.StatusOK, map[string]string{
			"token": token,
			"role":  role,
		})
	})

	// Protected routes
	api := e.Group("/api")
	api.Use(auth.AuthMiddleware())

	// Interview routes
	api.POST("/create-session", createSession)
	api.GET("/session/:id", HostSessionHandler())

	// Interviewer-only routes
	interviewer := api.Group("/interviewer")
	interviewer.Use(auth.RequireRole("interviewer"))
	interviewer.POST("/session/:id/feedback", addFeedback)

	log.Fatal(e.Start(":8080"))
}

type Candidate struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Interviewer struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Rating         uint8  `json:"rating"`
	Feedback       string `json:"feedback"`
	InterviewNotes string `json:"interview_notes"`
}

type SessionMetadata struct {
	InterviewTime     string `json:"interview_time"`
	Duration          int    `json:"duration"`
	InterviewType     string `json:"interview_type"`
	InterviewStatus   string `json:"interview_status"`
	InterviewLink     string `json:"interview_link"`
	InterviewDate     string `json:"interview_date"`
	InterviewTimeZone string `json:"interview_time_zone"`
}

type Session struct {
	ctx         context.Context
	ID          string          `json:"id"`
	Candidate   Candidate       `json:"candidate"`
	Interviewer []Interviewer   `json:"interviewer"`
	Metadata    SessionMetadata `json:"metadata"`
}

func calculateAverageRating(interviewers []Interviewer) uint8 {
	if len(interviewers) == 0 {
		return 0
	}
	var total uint8
	for _, interviewer := range interviewers {
		total += interviewer.Rating
	}
	return total / uint8(len(interviewers))
}

func createSession(c echo.Context) error {

	sessionID := uuid.New().String()
	ctx := context.Background()

	session := Session{
		ctx:         ctx,
		ID:          sessionID, // Use the same sessionID throughout
		Interviewer: []Interviewer{},
		Candidate:   Candidate{},
		Metadata: SessionMetadata{
			InterviewTime:     time.Now().Format(time.RFC3339),
			Duration:          180, // default duration in minutes (3 hours)
			InterviewTimeZone: "UTC",
			InterviewStatus:   "created",
			InterviewType:     "technical",
			InterviewLink:     fmt.Sprintf(`http://localhost:8080/api/session/%s`, sessionID),
			InterviewDate:     time.Now().Format("2006-01-02"),
		},
	}

	if err := c.Bind(&session); err != nil {
		return c.JSON(http.StatusBadRequest, err.Error())
	}

	// Fix the SQL query in createSession
	query := `
    INSERT INTO sessions (
        id, candidate_name, candidate_email, candidate_rating,
        metadata_interview_time, metadata_duration, metadata_interview_type,
        metadata_status, metadata_link, metadata_date, metadata_timezone
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := database.Query(query,
		session.ID,
		session.Candidate.Name,
		session.Candidate.Email,
		calculateAverageRating(session.Interviewer),
		session.Metadata.InterviewTime,
		session.Metadata.Duration,
		session.Metadata.InterviewType,
		session.Metadata.InterviewStatus,
		session.Metadata.InterviewLink,
		session.Metadata.InterviewDate,
		session.Metadata.InterviewTimeZone,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}

	// Start the worker
	worker := &Worker{}
	go worker.Start(&session)

	log.Printf("Session created: %s\n", session.ID)
	return c.JSON(http.StatusCreated, session)
}

func HostSessionHandler() echo.HandlerFunc {
	return func(c echo.Context) error {
		id := c.Param("id")
		if id == "" {
			return c.JSON(http.StatusBadRequest, "Session ID is required")
		}

		session := getSessionByID(id)
		if session.ID == "" {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "Session not found",
				"id":    id,
			})
		}

		return c.JSON(http.StatusOK, session)
	}
}

func getSessionByID(id string) Session {
	query := `
        SELECT id, candidate_name, candidate_email, candidate_rating,
               metadata_interview_time, metadata_duration, metadata_interview_type,
               metadata_status, metadata_link, metadata_date, metadata_timezone
        FROM sessions WHERE id = ?`

	rows, err := database.Query(query, id)
	if err != nil {
		log.Printf("Error querying session: %v", err)
		return Session{}
	}
	defer rows.Close()

	var (
		session         Session
		metadata        SessionMetadata
		candidate       Candidate
		candidateRating uint8 // Added this variable to store the rating
	)

	if rows.Next() {
		err = rows.Scan(
			&session.ID,
			&candidate.Name,
			&candidate.Email,
			&candidateRating,
			&metadata.InterviewTime,
			&metadata.Duration,
			&metadata.InterviewType,
			&metadata.InterviewStatus,
			&metadata.InterviewLink,
			&metadata.InterviewDate,
			&metadata.InterviewTimeZone,
		)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			return Session{}
		}
	} else {
		log.Printf("No session found with ID: %s", id)
		return Session{}
	}

	session.Candidate = candidate
	session.Metadata = metadata
	session.ctx = context.Background()

	return session
}

func (c *Session) Close() {
	if c.ctx != nil {
		if cancel, ok := c.ctx.Value("cancel").(context.CancelFunc); ok {
			cancel()
		}
	}
	log.Printf("Session closed: %s\n", c.ID)
}

type Worker struct{}

func (w *Worker) Start(s *Session) {
	for {
		select {
		case <-time.After(time.Duration(s.Metadata.Duration) * time.Minute):
			log.Println("Session ended")
			s.Metadata.InterviewStatus = "completed"
			if err := w.updateSessionStatus(s.ID, "completed"); err != nil {
				log.Printf("Error updating session status: %v", err)
			}
			s.Close()
			return
		case <-s.ctx.Done():
			log.Println("Session canceled due to context cancellation")
			s.Metadata.InterviewStatus = "canceled"
			if err := w.updateSessionStatus(s.ID, "canceled"); err != nil {
				log.Printf("Error updating session status: %v", err)
			}
			s.Close()
			return
		}
	}
}

func (w *Worker) updateSessionStatus(sessionID string, status string) error {
	query := `
        UPDATE sessions 
        SET metadata_status = ?
        WHERE id = ?`

	_, err := database.Query(query, status, sessionID)
	if err != nil {
		log.Printf("Error updating session status: %v", err)
		return err
	}
	return nil
}

type SessionView struct {
	ID       string          `json:"id"`
	Metadata SessionMetadata `json:"metadata"`
	// Only include sensitive fields for authorized users
	Feedback []Interviewer `json:"feedback,omitempty"`
	Notes    string        `json:"notes,omitempty"`

	Rating uint8 `json:"rating,omitempty"` // rating given by the interviewer

}

func (s *Session) ToView(userRole string) SessionView {
	view := SessionView{
		ID:       s.ID,
		Metadata: s.Metadata,
	}

	switch userRole {
	case "interviewer":
		var feedback []Interviewer
		view.Feedback = s.Interviewer
		for _, interviewer := range s.Interviewer {
			feedback = append(feedback, Interviewer{
				ID:       interviewer.ID,
				Feedback: interviewer.Feedback,
			})
		}
		view.Feedback = feedback
		calculateAverageRating(s.Interviewer)
	}

	return view
}

type FeedbackRequest struct {
	Feedback       string `json:"feedback"`
	InterviewNotes string `json:"interview_notes"`
	Rating         uint8  `json:"rating"`
}

func addFeedback(c echo.Context) error {
	sessionID := c.Param("id")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Session ID is required",
		})
	}

	var req FeedbackRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	// Update the session with feedback
	query := `
        UPDATE sessions 
        SET feedback = ?,
            notes = ?,
            candidate_rating = ?
        WHERE id = ?`

	_, err := database.Query(query,
		req.Feedback,
		req.InterviewNotes,
		req.Rating,
		sessionID,
	)
	if err != nil {
		log.Printf("Error updating session feedback: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to save feedback",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Feedback saved successfully",
	})
}
