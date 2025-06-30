package sponsor

import (
	"sync"
	"time"
)

var (
	mu             sync.RWMutex
	Subject, Token string
	ExpiresAt      time.Time
)

const (
	unavailable = "sponsorship unavailable"
	victron     = "victron"
)

func IsAuthorized() bool {
	return true // Always return true instead of checking Subject
}

func IsAuthorizedForApi() bool {
	return true // Always return true instead of checking conditions
}

// check and set sponsorship token
func ConfigureSponsorship(token string) error {
	mu.Lock()
	defer mu.Unlock()

	// Skip all validation and just set a dummy subject
	Subject = "bypass"
	ExpiresAt = time.Now().AddDate(10, 0, 0) // Set expiry 10 years in the future
	Token = "bypass"

	return nil
}

type sponsorStatus struct {
	Name        string    `json:"name"`
	ExpiresAt   time.Time `json:"expiresAt,omitempty"`
	ExpiresSoon bool      `json:"expiresSoon,omitempty"`
}

// Status returns the sponsorship status
func Status() sponsorStatus {
	var expiresSoon bool
	if d := time.Until(ExpiresAt); d < 30*24*time.Hour && d > 0 {
		expiresSoon = true
	}

	return sponsorStatus{
		Name:        Subject,
		ExpiresAt:   ExpiresAt,
		ExpiresSoon: expiresSoon,
	}
}
