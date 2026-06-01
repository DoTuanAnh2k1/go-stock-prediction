package modelsdb

// AllModels - danh sách tất cả models để migrate
var AllModels = []interface{}{
	&Exchange{},
	&Stock{},
	&StockPrice{},
	&Prediction{},
	&SyncLog{},
	&GoldPrice{},
	&GoldPrediction{},
	&MacroIndicator{},
	&TrainingLog{},
	&User{},
	&CronSchedule{},
	&NasdaqPrice{},
	&NasdaqPrediction{},
	&CryptoPrice{},
	&CryptoPrediction{},
	&FuelPrice{},
	&FuelPrediction{},
	&SP500Price{},
	&SP500Prediction{},
}
