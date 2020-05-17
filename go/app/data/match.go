package data

import (
	"github.com/muandrew/battlecode-legacy-go/models"
)

type Match struct {
	UUID        string
	BotUUIDs    []string
	MapUUID     string
	Winner      int
	Status      *models.BuildStatus
	Competition models.Competition
}

type Matches struct {
	Matches      []*models.Match
	TotalMatches int
}

func CreateMatch(match *models.Match) *Match {
	uuids := make([]string, len(match.Bots))
	for i, bot := range match.Bots {
		uuids[i] = bot.UUID
	}
	return &Match{
		match.UUID,
		uuids,
		match.MapUUID,
		match.Winner,
		match.Status,
		match.Competition,
	}
}
