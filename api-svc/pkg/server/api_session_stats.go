package server

import (
	"net/http"
	"strconv"
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

// sessionWindowAt computes the trading window for a given period/offset.
//   - period "week": a calendar week in ICT [Monday 00:00, next Monday 00:00),
//     stepped back `offset` weeks (applies to all markets).
//   - otherwise ("session"): the per-session window. offset 0 = current/most
//     recent session; offset>0 steps back that many calendar days (NASDAQ/SP500
//     use the 20:00→03:30 NYSE window, GOLD/CRYPTO use a full calendar day).
func sessionWindowAt(marketKey, period string, offset int, now time.Time) (start, end time.Time, isOpen bool) {
	if period == "week" {
		// Monday 00:00 of the current week (Go: Sunday=0 → shift so Monday=0).
		daysSinceMon := (int(now.Weekday()) + 6) % 7
		monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -daysSinceMon)
		start = monday.AddDate(0, 0, -7*offset)
		end = start.AddDate(0, 0, 7)
		isOpen = !now.Before(start) && now.Before(end)
		return
	}

	var bStart, bEnd time.Time
	var bOpen bool
	switch marketKey {
	case "NASDAQ", "SP500":
		bStart, bEnd, bOpen = nyseSessionWindow(now)
	default:
		bStart, bEnd, bOpen = dailySessionWindow(now)
	}
	if offset <= 0 {
		return bStart, bEnd, bOpen
	}
	// Past session: shift the base window back `offset` days; always closed.
	start = bStart.AddDate(0, 0, -offset)
	end = bEnd.AddDate(0, 0, -offset)
	isOpen = false
	return
}

// GetMarketSessionStats godoc
//
//	@Summary		Session stats for a market
//	@Description	Returns per-algorithm direction accuracy and bot trading stats for the current or most recent trading session.
//	@Tags			Markets
//	@Produce		json
//	@Param			key		path		string	true	"Market key: gold, nasdaq100, crypto, sp500"
//	@Param			period	query		string	false	"session (mặc định) hoặc week"
//	@Param			offset	query		int		false	"0 = phiên/tuần hiện tại, 1 = liền trước, ... (max 52)"
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

	// period: "session" (default) hoặc "week"; offset: 0 = hiện tại, 1 = liền trước, ...
	period := r.URL.Query().Get("period")
	if period != "week" {
		period = "session"
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 && v <= 52 {
			offset = v
		}
	}

	from, to, isOpen := sessionWindowAt(marketKey, period, offset, now)

	// Danh sách phiên/tuần gần đây để frontend render dropdown chọn.
	n := 10
	if period == "week" {
		n = 8
	}
	available := make([]modelsapi.SessionWindow, 0, n)
	for i := 0; i < n; i++ {
		s, e, open := sessionWindowAt(marketKey, period, i, now)
		available = append(available, modelsapi.SessionWindow{Offset: i, Start: s, End: e, IsOpen: open})
	}

	store := repository.GetSingleton()

	dirAcc, err := store.GetSessionDirAccuracy(r.Context(), marketKey, from, to)
	if err != nil {
		logger.Ctx(r.Context()).Errorf("[api/markets/%s/session-stats] GetSessionDirAccuracy: %v", key, err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get direction accuracy")
		return
	}
	if dirAcc == nil {
		dirAcc = []modelsapi.SessionDirAccRow{}
	}

	botTrades, err := store.GetSessionBotTrades(r.Context(), marketKey, from, to)
	if err != nil {
		logger.Ctx(r.Context()).Errorf("[api/markets/%s/session-stats] GetSessionBotTrades: %v", key, err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get bot trades")
		return
	}
	if botTrades == nil {
		botTrades = []modelsapi.SessionBotDetail{}
	}

	ResponseSuccess(w, http.StatusOK, modelsapi.SessionStatsResponse{
		Market: marketKey,
		Period: period,
		Session: modelsapi.SessionWindow{
			Offset: offset,
			Start:  from,
			End:    to,
			IsOpen: isOpen,
		},
		Available:         available,
		DirectionAccuracy: dirAcc,
		BotTrades:         botTrades,
	})
}
