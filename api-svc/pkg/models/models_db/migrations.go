package modelsdb

// AllModels - danh sách tất cả models để migrate
var AllModels = []interface{}{
	&SyncLog{},
	&GoldPrice{},
	&GoldPrediction{},
	&MacroIndicator{},
	&TrainingLog{},
	&CronSchedule{},
	&NasdaqPrice{},
	&NasdaqPrediction{},
	&CryptoPrice{},
	&CryptoPrediction{},
	&SP500Price{},
	&SP500Prediction{},
	&NasdaqIntradayPrice{},
	&SP500IntradayPrice{},
	&CryptoIntradayPrice{},
	&GoldIntradayPrice{},
	&SimBot{},
	&SimSession{},
	&SimTrade{},
	&SimPortfolioSnapshot{},
}
