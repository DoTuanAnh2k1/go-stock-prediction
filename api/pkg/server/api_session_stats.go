package server

import (
	"net/http"
	"time"

	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
)

// nyseSessionWindow returns the start, end, and open status of the current or
// most recent NYSE trading session in the given time's timezone (ICT).
//
// NYSE hours: 9:00AM–4:30PM ET = 20:00–03:30 ICT (spans two calendar days).
func nyseSessionWindow(now time.Time) (start, end time.Time, isOpen bool) {
	h, m := now.Hour(), now.Minute()

	if h >= 20 {
		// Session opened today at 20:00, closes tomorrow at 03:30
		start = time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, now.Location())
		end = start.Add(7*time.Hour + 30*time.Minute)
		isOpen = true
	} else if h < 3 || (h == 3 && m < 30) {
		// Session opened yesterday at 20:00, closes today at 03:30
		yesterday := now.AddDate(0, 0, -1)
		start = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 20, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), now.Day(), 3, 30, 0, 0, now.Location())
		isOpen = true
	} else {
		// 03:30–20:00: between sessions — show most recent completed session
		yesterday := now.AddDate(0, 0, -1)
		start = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 20, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), now.Day(), 3, 30, 0, 0, now.Location())
		isOpen = false
	}
	return
}

// dailySessionWindow returns the current 24/7 trading session for markets that
// never close (GOLD, CRYPTO): a full calendar day in ICT, from the most recent
// midnight (00:00 today) to the nearest next midnight (00:00 tomorrow).
// The session is always considered open since these markets trade non-stop.
func dailySessionWindow(now time.Time) (start, end time.Time, isOpen bool) {
	start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end = start.AddDate(0, 0, 1)
	isOpen = true
	return
}

// GetMarketSessionStats godoc
//
//	@Summary		Session stats for a market
//	@Description	Returns per-algorithm direction accuracy and bot trading stats for the current or most recent trading session.
//	@Tags			Markets
//	@Produce		json
//	@Param			key	path		string	true	"Market key: gold, nasdaq100, crypto, sp500"
//	@Success		200	{object}	modelsapi.SessionStatsResponse
//	@Failure		403	{object}	ResponseFailure
//	@Failure		500	{object}	ResponseFailure
//	@Router			/api/markets/{key}/session-stats [get]
func GetMarketSessionStats(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	marketKey := pathToMarketKey(key)
	if !checkMarketAccess(w, r, marketKey) {
		return
	}

	now := time.Now()
	var from, to time.Time
	var isOpen bool

	switch marketKey {
	case "NASDAQ", "SP500":
		from, to, isOpen = nyseSessionWindow(now)
	default:
		// GOLD, CRYPTO: trade 24/7 — one session is a full calendar day,
		// from today 00:00 to the nearest next midnight (tomorrow 00:00).
		from, to, isOpen = dailySessionWindow(now)
	}

	store := repository.GetSingleton()

	dirAcc, err := store.GetSessionDirAccuracy(marketKey, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/markets/%s/session-stats] GetSessionDirAccuracy: %v", key, err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get direction accuracy")
		return
	}
	if dirAcc == nil {
		dirAcc = []modelsapi.SessionDirAccRow{}
	}

	botTrades, err := store.GetSessionBotTrades(marketKey, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/markets/%s/session-stats] GetSessionBotTrades: %v", key, err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get bot trades")
		return
	}
	if botTrades == nil {
		botTrades = []modelsapi.SessionBotDetail{}
	}

	ResponseSuccess(w, http.StatusOK, modelsapi.SessionStatsResponse{
		Market: marketKey,
		Session: modelsapi.SessionWindow{
			Start:  from,
			End:    to,
			IsOpen: isOpen,
		},
		DirectionAccuracy: dirAcc,
		BotTrades:         botTrades,
	})
}
