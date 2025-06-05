package modelssvc

import (
	"go-stock-prediction/pkg/utils/parse"
	"time"

	"github.com/gocolly/colly/v2"
)

type VN30Stock struct {
	Symbol        string    `json:"symbol"`         // Mã CK: VCB, VIC, etc.
	Price         float64   `json:"price"`          // Giá hiện tại
	Change        float64   `json:"change"`         // Thay đổi
	ChangePercent float64   `json:"change_percent"` // % thay đổi
	Volume        int64     `json:"volume"`         // Khối lượng GD
	Value         int64     `json:"value"`          // Giá trị GD (VNĐ)
	High          float64   `json:"high"`           // Giá cao nhất
	Low           float64   `json:"low"`            // Giá thấp nhất
	Open          float64   `json:"open"`           // Giá mở cửa
	Timestamp     time.Time `json:"timestamp"`      // Thời gian
}

func (vs *VietStockScraper) ExtractVN30Data() ([]VN30Stock, error) {
	var stocks []VN30Stock

	vs.collector.OnHTML("table.stock-table tr", func(e *colly.HTMLElement) {
		// Bỏ qua header
		if e.ChildText("th") != "" {
			return
		}

		stock := VN30Stock{
			Symbol:        e.ChildText("td:nth-child(1)"),
			Price:         parse.ParseFloat(e.ChildText("td:nth-child(2)")),
			Change:        parse.ParseFloat(e.ChildText("td:nth-child(3)")),
			ChangePercent: parse.ParseFloat(e.ChildText("td:nth-child(4)")),
			Volume:        parse.ParseInt64(e.ChildText("td:nth-child(5)")),
			High:          parse.ParseFloat(e.ChildText("td:nth-child(7)")),
			Low:           parse.ParseFloat(e.ChildText("td:nth-child(8)")),
			Open:          parse.ParseFloat(e.ChildText("td:nth-child(9)")),
			Timestamp:     time.Now(),
		}
		stocks = append(stocks, stock)
	})

	return stocks, nil
}
