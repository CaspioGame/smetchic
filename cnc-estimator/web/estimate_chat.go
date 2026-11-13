package web

import (
	"strings"
	"time"
)

type estimateChatTurn struct {
	ID, CompanyID, EstimateID, VersionID string
	Message, Reply                       string
	Changes                              []string
	Proposal                             estimateAssistantReply
	Pending                              bool
	BaseInput                            string
}

func (store *sessionStore) estimateChatTurns(sessionID, companyID, estimateID string) []estimateChatTurn {
	store.mu.Lock()
	defer store.mu.Unlock()
	var turns []estimateChatTurn
	for _, turn := range store.sessions[sessionID].estimateChat {
		if turn.CompanyID == companyID && turn.EstimateID == estimateID {
			turns = append(turns, turn)
		}
	}
	return turns
}

func (store *sessionStore) addEstimateChatTurn(sessionID string, turn estimateChatTurn) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.sessions[sessionID]
	if !ok || !time.Now().Before(session.ExpiresAt) {
		return
	}
	const maxTurns = 30
	session.estimateChat = append(session.estimateChat, turn)
	if len(session.estimateChat) > maxTurns {
		session.estimateChat = append([]estimateChatTurn(nil), session.estimateChat[len(session.estimateChat)-maxTurns:]...)
	}
	store.sessions[sessionID] = session
}

func estimateChatPrompt(turns []estimateChatTurn, message string) string {
	var prompt strings.Builder
	for _, turn := range turns[max(0, len(turns)-2):] {
		prompt.WriteString("Ранее пользователь: " + turn.Message + "\n")
		prompt.WriteString("Ранее помощник: " + turn.Reply + "\n")
	}
	prompt.WriteString("Текущий запрос: " + message)
	return prompt.String()
}
